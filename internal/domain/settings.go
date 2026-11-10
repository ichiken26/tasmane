package domain

// ユーザー設定のドメイン値
type UserSettings struct {
	Timezone                  string
	GoogleSyncIntervalMinutes int
	MemoIntegrationEnabled    bool
}
