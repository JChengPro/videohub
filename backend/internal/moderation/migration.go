package moderation

import (
	"backend/internal/account"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func lockStaffState(tx *gorm.DB) error {
	var state StaffState
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, 1).Error
}

func MigrateStaff(db *gorm.DB) error {
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&StaffState{ID: 1}).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockStaffState(tx); err != nil {
			return err
		}
		var state StaffState
		if err := tx.First(&state, 1).Error; err != nil {
			return err
		}
		if state.Migrated {
			return nil
		}
		var legacy []account.Account
		if err := tx.Where("role = ?", "admin").Order("id").Find(&legacy).Error; err != nil {
			return err
		}
		for _, user := range legacy {
			member := Staff{ID: user.ID, AccountName: user.AccountName, Username: user.Username, Password: user.Password, Role: "owner", Status: "active"}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
			if err := audit(tx, Staff{Username: "系统迁移"}, member, "migration", "admin", "owner/active"); err != nil {
				return err
			}
		}
		// Legacy sessions used community IDs. Require a fresh login after migration.
		if err := tx.Where("1 = 1").Delete(&AdminSession{}).Error; err != nil {
			return err
		}
		return tx.Model(&state).Update("migrated", true).Error
	})
}
