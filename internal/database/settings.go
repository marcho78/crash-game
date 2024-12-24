package database

import (
	"crash-game/internal/models"
)

func (d *Database) GetUserSettings(userID string) (*models.UserSettings, error) {
	settings := &models.UserSettings{}
	err := d.db.QueryRow(`
        SELECT 
            theme,
            sound_enabled,
            email_notifications,
            auto_cashout_enabled,
            auto_cashout_value,
            language,
            timezone
        FROM user_settings
        WHERE user_id = $1`,
		userID).Scan(
		&settings.Theme,
		&settings.SoundEnabled,
		&settings.EmailNotifications,
		&settings.AutoCashoutEnabled,
		&settings.AutoCashoutValue,
		&settings.Language,
		&settings.Timezone,
	)

	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (d *Database) UpdateUserSettings(userID string, settings *models.UserSettings) error {
	_, err := d.db.Exec(`
        INSERT INTO user_settings (
            user_id,
            theme,
            sound_enabled,
            email_notifications,
            auto_cashout_enabled,
            auto_cashout_value,
            language,
            timezone
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (user_id) DO UPDATE SET
            theme = EXCLUDED.theme,
            sound_enabled = EXCLUDED.sound_enabled,
            email_notifications = EXCLUDED.email_notifications,
            auto_cashout_enabled = EXCLUDED.auto_cashout_enabled,
            auto_cashout_value = EXCLUDED.auto_cashout_value,
            language = EXCLUDED.language,
            timezone = EXCLUDED.timezone,
            updated_at = CURRENT_TIMESTAMP`,
		userID,
		settings.Theme,
		settings.SoundEnabled,
		settings.EmailNotifications,
		settings.AutoCashoutEnabled,
		settings.AutoCashoutValue,
		settings.Language,
		settings.Timezone,
	)

	return err
}
