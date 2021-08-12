package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	c "github.com/chollinger93/telegram-camera-bridge/core"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the service",
	Long:  `Start the integration service between MotionEye OS and Telegram.`,
	Run: func(cmd *cobra.Command, args []string) {
		app := newApp()
		// Rest
		app.Serve()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type App struct {
	Router  *mux.Router
	Cfg     *c.Config
	RestUri string
	// Modules
	Snapshots           c.CamModule
	SnapshotTimeoutChan chan int
}

func newApp() *App {
	app := &App{}

	// Read the config
	cfg := &c.Config{}
	err := viper.ReadInConfig()
	if err != nil {
		zap.S().Fatalf("Cannot read config")
	}
	err = viper.Unmarshal(cfg)
	if err != nil {
		zap.S().Fatalf("Cannot read config")
	}
	// TODO: validate
	app.Cfg = cfg
	// Modules
	if app.Cfg.Snapshots.Enabled {
		bot, err := tgbotapi.NewBotAPI(cfg.Telegram.ApiKey)
		if err != nil {
			zap.S().Fatalf("Err creating Telegram bot: %v", err)
		}

		app.Snapshots = &c.Snapshots{
			Cfg:    app.Cfg,
			Client: &http.Client{},
			TgBot:  bot,
		}

		app.SnapshotTimeoutChan = make(chan int, 1)
	}

	// Build a router
	app.buildRouter()

	return app
}

func (app *App) buildRouter() {
	app.Router = mux.NewRouter()
	if app.Cfg.General.RateFilter != 0 {
		// Throttling
		store, err := memstore.New(65536)
		if err != nil {
			log.Fatal(err)
		}

		quota := throttled.RateQuota{
			MaxRate:  throttled.PerHour(app.Cfg.General.RateFilter),
			MaxBurst: 0,
		}
		rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
		if err != nil {
			zap.S().Fatal(err)
		}

		httpRateLimiter := throttled.HTTPRateLimiter{
			Error:       app.denyHandler,
			RateLimiter: rateLimiter,
			VaryBy:      &throttled.VaryBy{Path: true},
		}
		// Handle
		app.Router.Use(httpRateLimiter.RateLimit)
		app.Router.HandleFunc("/motion", app.PostHandlerMotion).Methods(http.MethodPost)
		http.Handle("/", httpRateLimiter.RateLimit(app.Router))

	} else {
		app.Router.HandleFunc("/motion", app.PostHandlerMotion).Methods(http.MethodPost)
		http.Handle("/", app.Router)
	}
}

func (a *App) denyHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write(nil)
}

func (a *App) sendErr(w http.ResponseWriter, msg string, code int) {
	response, _ := json.Marshal(msg)
	w.WriteHeader(code)
	w.Write(response)
}

func (a *App) filterRequest(w http.ResponseWriter, r *http.Request) error {
	addr := c.GetRealAddr(r)
	// TODO: empty/null
	if addr == "" && a.Cfg.General.IpFilter != "" {
		zap.S().Error("Cannot get IP from request")
		http.Error(w, "Blocked", 401)
		return fmt.Errorf("Cannot get IP from request")
	}

	if a.Cfg.General.IpFilter != "" {
		zap.S().Warnf("Checking IP %s against %s", addr, a.Cfg.General.IpFilter)
		if !strings.HasPrefix(addr, a.Cfg.General.IpFilter) {
			http.Error(w, "Blocked", 401)
			return fmt.Errorf("Blocked")
		}
	}
	return nil
}

func isWithinActiveHours(now, start, end string) bool {
	startTime, err := time.Parse("15:04", start)
	if err != nil {
		zap.S().Error(err)
		return false
	}
	endTime, err := time.Parse("15:04", end)
	if err != nil {
		zap.S().Error(err)
		return false
	}
	nowTime, err := time.Parse("15:04", now)
	if err != nil {
		zap.S().Error(err)
		return false
	}
	return inTimeSpan(nowTime, startTime, endTime)
}

func inTimeSpan(check, start, end time.Time) bool {
	if start.Before(end) {
		return !check.Before(start) && !check.After(end)
	}
	if start.Equal(end) {
		return check.Equal(start)
	}
	return !start.After(check) || !end.Before(check)
}

func getNowAsHrMinString() string {
	now := time.Now()
	return fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
}

func (a *App) handleSnapshots() error {
	// Time filter
	if !isWithinActiveHours(getNowAsHrMinString(), a.Cfg.Snapshots.ActiveTime.FromTime, a.Cfg.Snapshots.ActiveTime.ToTime) {
		err := fmt.Errorf("Outside active hours (%s / %s - %s), ignoring", getNowAsHrMinString(), a.Cfg.Snapshots.ActiveTime.FromTime, a.Cfg.Snapshots.ActiveTime.ToTime)
		zap.S().Warn(err)
		return err
	}

	data, err := a.Snapshots.Capture()
	if err != nil {
		return err
	}
	err = a.Snapshots.Send(data)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) PostHandlerMotion(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		zap.S().Error("Null request")
		a.sendErr(w, "", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// IP range filter
	if err := a.filterRequest(w, r); err != nil {
		return
	}
	_, err := ioutil.ReadAll(r.Body)
	if err != nil {
		zap.S().Error(err)
		a.sendErr(w, "", http.StatusBadRequest)
		return
	}
	// Handle Snapshots
	if a.Cfg.Snapshots.Enabled {
		err = a.handleSnapshots()
		if err != nil {
			zap.S().Error(err)
			a.sendErr(w, "", http.StatusInternalServerError)
		}
	}
}

func (a *App) periodicUpdater() {
	// Snapshots
	if a.Cfg.Snapshots.Enabled {
		interval := a.Cfg.Snapshots.IntervalS
		zap.S().Infof("Snapshot updater running every %vs", interval)
		ticker := time.NewTicker(time.Second * time.Duration(interval))

		for {
			select {
			case <-a.SnapshotTimeoutChan:
				zap.S().Infof("Updated Snapshot updater running every %vs", a.Cfg.Snapshots.IntervalS)
				ticker.Stop()
				ticker = time.NewTicker(time.Second * time.Duration(a.Cfg.Snapshots.IntervalS))
			case <-ticker.C:
				zap.S().Debugf("Tick, taking screenshot after %vs", a.Cfg.Snapshots.IntervalS)
				go a.handleSnapshots()
			}

		}
	}
}

func (a *App) commandHandler() {
	if a.Cfg.Snapshots.Enabled {
		go a.Snapshots.HandleCommands(a.SnapshotTimeoutChan)
	}
}

func (a *App) Serve() {
	// Update snapshots and videos
	go a.periodicUpdater()
	// Handle commands
	go a.commandHandler()

	// Server
	uri := fmt.Sprintf("%s:%s", a.Cfg.General.Server, a.Cfg.General.Port)
	zap.S().Infof("Listening on %s", uri)
	a.RestUri = uri
	log.Fatal(http.ListenAndServe(uri, a.Router))
}
