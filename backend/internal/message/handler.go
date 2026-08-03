package message

import (
	"backend/internal/realtime"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

type Handler struct {
	service    *Service
	tickets    *realtime.TicketService
	hub        *realtime.Hub
	dispatcher *realtime.Dispatcher
}

func NewHandler(
	service *Service,
	tickets *realtime.TicketService,
	hub *realtime.Hub,
	dispatcher *realtime.Dispatcher,
) *Handler {
	return &Handler{
		service:    service,
		tickets:    tickets,
		hub:        hub,
		dispatcher: dispatcher,
	}
}

func currentAccountID(c *gin.Context) (uint, bool) {
	value, ok := c.Get("accountID")
	if !ok {
		return 0, false
	}
	id, ok := value.(uint)
	return id, ok && id > 0
}

func (h *Handler) ListConversations(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	response, err := h.service.ListConversations(c.Request.Context(), accountID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) ListMessages(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	var request ListMessagesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response, err := h.service.ListMessages(c.Request.Context(), accountID, request)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Send(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	var request SendRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response, err := h.service.Send(c.Request.Context(), accountID, request)
	if err != nil {
		writeError(c, err)
		return
	}
	h.publishSentMessage(c.Request.Context(), response)
	c.JSON(http.StatusOK, response)
}

func (h *Handler) MarkRead(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	var request MarkReadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.MarkRead(c.Request.Context(), accountID, request)
	if err != nil {
		writeError(c, err)
		return
	}
	h.publishReadReceipt(c.Request.Context(), accountID, result)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Accept(c *gin.Context) {
	h.conversationAction(c, StatusAccepted)
}

func (h *Handler) Reject(c *gin.Context) {
	h.conversationAction(c, StatusRejected)
}

func (h *Handler) conversationAction(c *gin.Context, status string) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	var request ConversationActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var (
		peerID uint
		err    error
	)
	if status == StatusAccepted {
		peerID, err = h.service.Accept(c.Request.Context(), accountID, request.ConversationID)
	} else {
		peerID, err = h.service.Reject(c.Request.Context(), accountID, request.ConversationID)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	event := realtime.NewEvent("chat.conversation_changed", map[string]any{
		"conversation_id": request.ConversationID,
		"status":          status,
	})
	h.dispatcher.Publish(c.Request.Context(), accountID, event)
	h.dispatcher.Publish(c.Request.Context(), peerID, event)
	c.JSON(http.StatusOK, gin.H{"message": "conversation updated", "status": status})
}

func (h *Handler) Block(c *gin.Context) {
	h.userAction(c, true)
}

func (h *Handler) Unblock(c *gin.Context) {
	h.userAction(c, false)
}

func (h *Handler) userAction(c *gin.Context, block bool) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	var request UserActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var err error
	if block {
		err = h.service.Block(c.Request.Context(), accountID, request.UserID)
	} else {
		err = h.service.Unblock(c.Request.Context(), accountID, request.UserID)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	status := "unblocked"
	if block {
		status = StatusBlocked
	}
	event := realtime.NewEvent("chat.conversation_changed", map[string]any{
		"peer_id": request.UserID,
		"status":  status,
	})
	h.dispatcher.Publish(c.Request.Context(), accountID, event)
	h.dispatcher.Publish(c.Request.Context(), request.UserID, event)
	c.JSON(http.StatusOK, gin.H{"message": status})
}

func (h *Handler) UnreadCount(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	count, err := h.service.UnreadCount(c.Request.Context(), accountID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, UnreadCountResponse{Count: count})
}

func (h *Handler) IssueTicket(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid account id"})
		return
	}
	ticket, ttl, err := h.tickets.Issue(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ticket":     ticket,
		"expires_in": int(ttl.Seconds()),
	})
}

func (h *Handler) WebSocket(c *gin.Context) {
	accountID, err := h.tickets.Consume(c.Request.Context(), strings.TrimSpace(c.Query("ticket")))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	server := websocket.Server{
		Handler: func(socket *websocket.Conn) {
			h.hub.Serve(accountID, socket, h.handleIncoming)
		},
		Handshake: validateOrigin,
	}
	server.ServeHTTP(c.Writer, c.Request)
}

func (h *Handler) handleIncoming(
	ctx context.Context,
	accountID uint,
	incoming realtime.IncomingEvent,
) *realtime.Event {
	switch incoming.Type {
	case "chat.send":
		var request SendRequest
		if err := json.Unmarshal(incoming.Data, &request); err != nil {
			event := realtime.ErrorEvent(incoming.RequestID, "INVALID_MESSAGE", "消息格式错误")
			return &event
		}
		response, err := h.service.Send(ctx, accountID, request)
		if err != nil {
			event := websocketError(incoming.RequestID, err)
			return &event
		}
		h.publishSentMessage(ctx, response)
		event := realtime.NewEvent("chat.message_ack", response)
		event.RequestID = incoming.RequestID
		return &event

	case "chat.mark_read":
		var request MarkReadRequest
		if err := json.Unmarshal(incoming.Data, &request); err != nil {
			event := realtime.ErrorEvent(incoming.RequestID, "INVALID_READ_REQUEST", "已读请求格式错误")
			return &event
		}
		result, err := h.service.MarkRead(ctx, accountID, request)
		if err != nil {
			event := websocketError(incoming.RequestID, err)
			return &event
		}
		h.publishReadReceipt(ctx, accountID, result)
		event := realtime.NewEvent("chat.read_ack", result)
		event.RequestID = incoming.RequestID
		return &event
	default:
		event := realtime.ErrorEvent(incoming.RequestID, "UNSUPPORTED_EVENT", "不支持的 WebSocket 事件")
		return &event
	}
}

func (h *Handler) publishSentMessage(ctx context.Context, response SendResponse) {
	if response.Idempotent {
		return
	}
	h.dispatcher.Publish(ctx, response.Message.ReceiverID, realtime.NewEvent("chat.new_message", response))
	h.dispatcher.Publish(ctx, response.Message.SenderID, realtime.NewEvent("chat.conversation_changed", map[string]any{
		"conversation_id": response.Message.ConversationID,
		"status":          response.ConversationStatus,
	}))
}

func (h *Handler) publishReadReceipt(ctx context.Context, accountID uint, result MarkReadResult) {
	h.dispatcher.Publish(ctx, result.PeerID, realtime.NewEvent("chat.read_receipt", map[string]any{
		"conversation_id": result.ConversationID,
		"reader_id":       accountID,
		"message_id":      result.ReadMessageID,
	}))
	h.dispatcher.Publish(ctx, accountID, realtime.NewEvent("chat.unread_count", map[string]any{
		"count": result.UnreadCount,
	}))
}

func writeError(c *gin.Context, err error) {
	var policy *PolicyError
	if errors.As(err, &policy) {
		status := http.StatusForbidden
		switch policy.Code {
		case "MESSAGE_RATE_LIMIT":
			status = http.StatusTooManyRequests
		case "RECEIVER_NOT_FOUND", "USER_NOT_FOUND", "CONVERSATION_NOT_FOUND":
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": policy.Message, "code": policy.Code})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func websocketError(requestID string, err error) realtime.Event {
	var policy *PolicyError
	if errors.As(err, &policy) {
		return realtime.ErrorEvent(requestID, policy.Code, policy.Message)
	}
	return realtime.ErrorEvent(requestID, "MESSAGE_ERROR", err.Error())
}

func validateOrigin(config *websocket.Config, request *http.Request) error {
	rawOrigin := strings.TrimSpace(request.Header.Get("Origin"))
	if rawOrigin == "" {
		return errors.New("missing websocket origin")
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil || origin.Hostname() == "" {
		return errors.New("invalid websocket origin")
	}
	requestHost := request.Host
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-Host")); forwarded != "" {
		requestHost = strings.Split(forwarded, ",")[0]
	}
	host, _, err := net.SplitHostPort(requestHost)
	if err != nil {
		host = requestHost
	}
	if !strings.EqualFold(origin.Hostname(), host) {
		return errors.New("websocket origin is not allowed")
	}
	config.Origin = origin
	return nil
}
