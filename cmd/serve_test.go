package cmd

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	c "github.com/chollinger93/telegram-camera-bridge/core"
	"github.com/stretchr/testify/assert"
)

func Test_isWithinActiveHours(t *testing.T) {
	baseDt := time.Date(1776, 7, 4, 12, 0, 0, 0, time.UTC)
	type args struct {
		now   string
		start string
		end   string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "isWithin",
			args: args{
				now:   fmt.Sprintf("%02d:%02d", baseDt.Hour(), baseDt.Minute()),
				start: "08:00",
				end:   "18:00",
			},
			want: true,
		},
		{
			name: "isOutside",
			args: args{
				now:   fmt.Sprintf("%02d:%02d", baseDt.Hour(), baseDt.Minute()),
				start: "14:00",
				end:   "18:00",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWithinActiveHours(tt.args.now, tt.args.start, tt.args.end); got != tt.want {
				t.Errorf("isWithinActiveHours() = %v, want %v", got, tt.want)
			}
		})
	}
}

type MockCamModule struct {
	Cfg *c.Config
}

func (a *MockCamModule) Capture() (image.Image, error) {
	return nil, nil
}

func (a *MockCamModule) SendTo(img image.Image, replyId int) error {
	return nil
}

func (a *MockCamModule) Send(img image.Image) error {
	return a.SendTo(img, 0)
}

func (a *MockCamModule) HandleCommands(ch chan int) error {
	return nil
}

func executeRequest(a *App, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	a.Router.ServeHTTP(rr, req)

	return rr
}

var defaultCfg = &c.Config{
	General: c.GeneralConfig{
		Server:     "127.0.0.1",
		Port:       "5060",
		RateFilter: 0,
	},
	Telegram: c.TelegramConfig{
		BotName: "@unit_test",
		ApiKey:  "mock",
		ChatId:  -1,
	},
	Snapshots: c.SnapshotConfig{
		Enabled:   true,
		IntervalS: 60,
		ActiveTime: c.TimerConfig{
			FromTime: "00:00",
			ToTime:   "23:59",
		},
		SnapshotUrl: "http://127.0.0.1",
	},
}

func TestApp_handleSnapshots(t *testing.T) {
	var a App = App{SnapshotTimeoutChan: make(chan int, 1)}

	mockJson := []byte(`{}`)

	tests := []struct {
		name        string
		Cfg         *c.Config
		requestBody io.Reader
		wantCode    int
		wantErr     bool
	}{
		{
			name:        "Valid request",
			Cfg:         defaultCfg,
			requestBody: bytes.NewBuffer(mockJson),
			wantCode:    200,
			wantErr:     false,
		},
		{
			name:        "Null request",
			Cfg:         defaultCfg,
			requestBody: nil,
			wantCode:    400,
			wantErr:     false,
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Finalize mock
			a.Cfg = tt.Cfg
			a.Snapshots = &MockCamModule{Cfg: tt.Cfg}

			if i == 0 {
				//Build router only once
				a.buildRouter()
			}

			// Tests
			req, _ := http.NewRequest("POST", "/motion", tt.requestBody)
			resp := executeRequest(&a, req)
			assert.Equal(t, tt.wantCode, resp.Code)
			// Reset thy handlers
			http.DefaultServeMux = new(http.ServeMux)
		})
	}
}

func TestApp_handleSnapshots_RateLimited(t *testing.T) {
	var a App = App{SnapshotTimeoutChan: make(chan int, 1)}

	mockJson := []byte(`{}`)

	tests := []struct {
		name        string
		Cfg         *c.Config
		requestBody io.Reader
		wantCode    int
		wantErr     bool
	}{
		{
			name:        "Rate limited",
			Cfg:         defaultCfg,
			requestBody: bytes.NewBuffer(mockJson),
			wantCode:    200,
			wantErr:     false,
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Finalize mock
			a.Cfg = tt.Cfg
			a.Cfg.General.RateFilter = 1 // Max 1 request per hr
			a.Snapshots = &MockCamModule{Cfg: tt.Cfg}

			if i == 0 {
				//Build router only once
				a.buildRouter()
			}

			// Tests
			for i := 1; i <= 10; i++ {
				req, _ := http.NewRequest("POST", "/motion", tt.requestBody)
				resp := executeRequest(&a, req)
				if i == 1 {
					assert.Equal(t, tt.wantCode, resp.Code)
				} else {
					assert.Equal(t, 429, resp.Code)
				}
			}

			http.DefaultServeMux = new(http.ServeMux)
		})
	}
}
