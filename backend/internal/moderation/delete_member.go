package moderation

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) DeleteMember(c *gin.Context) {
	var input struct {
		ID          uint   `json:"id"`
		AccountName string `json:"account_name"`
	}
	if c.ShouldBindJSON(&input) != nil || input.ID == 0 || strings.TrimSpace(input.AccountName) == "" {
		c.JSON(400, gin.H{"error": "请输入待删除成员的登录账号确认"})
		return
	}
	err := h.staffTransaction(c, true, func(tx *gorm.DB, actor Staff) error {
		var member Staff
		if err := tx.Where("status <> ?", "deleted").First(&member, input.ID).Error; err != nil {
			return err
		}
		if member.ID == actor.ID {
			return problem(409, "不能删除当前登录账号")
		}
		if member.AccountName != strings.ToLower(strings.TrimSpace(input.AccountName)) {
			return problem(400, "确认账号与待删除成员不一致")
		}
		if err := protectLastOwner(tx, member, member.Role, "deleted"); err != nil {
			return err
		}
		before := member.Role + "/" + member.Status
		// Keep the identity and historical references, but permanently remove access credentials.
		if err := tx.Model(&member).Updates(map[string]any{"status": "deleted", "deleted_at": time.Now(), "password": ""}).Error; err != nil {
			return err
		}
		if err := invalidate(tx, member.ID); err != nil {
			return err
		}
		if err := removeLink(tx, member.ID); err != nil {
			return err
		}
		return audit(tx, actor, member, "delete", before, member.Role+"/deleted")
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "成员已删除，历史记录已保留"})
}
