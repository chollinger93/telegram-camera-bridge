package core

type Config struct {
	General   GeneralConfig
	Telegram  TelegramConfig
	Snapshots SnapshotConfig
	Videos    VideoConfig
}

type GeneralConfig struct {
	Server   string
	Port     string
	IpFilter string `mapstructure:"ip_filter"`
}

type SnapshotConfig struct {
	Enabled     bool
	IntervalS   int         `mapstructure:"interval_s"`
	ActiveTime  TimerConfig `mapstructure:"active_time"`
	SnapshotUrl string      `mapstructure:"snapshot_url"`
}

type VideoConfig struct {
	Enabled    bool
	IntervalS  int         `mapstructure:"interval_s"`
	LengthS    int         `mapstructure:"length_s"`
	ActiveTime TimerConfig `mapstructure:"active_time"`
}

type TimerConfig struct {
	FromTime string `mapstructure:"from_time"`
	ToTime   string `mapstructure:"to_time"`
}

type TelegramConfig struct {
}
