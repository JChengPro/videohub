package message

import "testing"

func TestExplicitTableNames(t *testing.T) {
	if got := (Conversation{}).TableName(); got != conversationTableName {
		t.Fatalf("conversation table name = %q, want %q", got, conversationTableName)
	}
	if got := (Message{}).TableName(); got != messageTableName {
		t.Fatalf("message table name = %q, want %q", got, messageTableName)
	}
	if got := (Block{}).TableName(); got != blockTableName {
		t.Fatalf("block table name = %q, want %q", got, blockTableName)
	}
}

func TestRemainingMessages(t *testing.T) {
	tests := []struct {
		sent uint8
		want uint8
	}{
		{sent: 0, want: 3},
		{sent: 1, want: 2},
		{sent: 2, want: 1},
		{sent: 3, want: 0},
		{sent: 4, want: 0},
	}
	for _, test := range tests {
		got := remainingMessages(Conversation{RequestSentCount: test.sent})
		if got != test.want {
			t.Fatalf("sent=%d: got remaining=%d, want=%d", test.sent, got, test.want)
		}
	}
}

func TestEffectivePolicy(t *testing.T) {
	const (
		requester = uint(1)
		receiver  = uint(2)
	)
	tests := []struct {
		name         string
		conversation Conversation
		accountID    uint
		mutual       bool
		blocked      bool
		wantStatus   string
		wantSend     bool
		wantReply    bool
	}{
		{
			name: "requester has quota",
			conversation: Conversation{
				Status: StatusPending, RequestSenderID: requester, RequestSentCount: 2,
			},
			accountID: requester, wantStatus: StatusPending, wantSend: true,
		},
		{
			name: "requester exhausted quota",
			conversation: Conversation{
				Status: StatusPending, RequestSenderID: requester, RequestSentCount: 3,
			},
			accountID: requester, wantStatus: StatusPending,
		},
		{
			name: "receiver can reply and accept",
			conversation: Conversation{
				Status: StatusPending, RequestSenderID: requester, RequestSentCount: 3,
			},
			accountID: receiver, wantStatus: StatusPending, wantSend: true, wantReply: true,
		},
		{
			name: "accepted conversation",
			conversation: Conversation{
				Status: StatusAccepted, RequestSenderID: requester,
			},
			accountID: requester, wantStatus: StatusAccepted, wantSend: true, wantReply: true,
		},
		{
			name: "mutual overrides pending quota",
			conversation: Conversation{
				Status: StatusPending, RequestSenderID: requester, RequestSentCount: 3,
			},
			accountID: requester, mutual: true, wantStatus: StatusMutual, wantSend: true, wantReply: true,
		},
		{
			name: "block overrides mutual",
			conversation: Conversation{
				Status: StatusAccepted, RequestSenderID: requester,
			},
			accountID: requester, mutual: true, blocked: true, wantStatus: StatusBlocked,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, canSend, canReply := effectivePolicy(
				test.conversation,
				test.accountID,
				test.mutual,
				test.blocked,
			)
			if status != test.wantStatus || canSend != test.wantSend || canReply != test.wantReply {
				t.Fatalf(
					"got (%s,%v,%v), want (%s,%v,%v)",
					status, canSend, canReply,
					test.wantStatus, test.wantSend, test.wantReply,
				)
			}
		})
	}
}
