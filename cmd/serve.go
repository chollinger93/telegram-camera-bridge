package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	c "github.com/chollinger93/telegram-camera-bridge/core"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

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
		newApp().Serve()
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
	Router *mux.Router
	Cfg    *c.Config
	// Modules
	Snapshots c.CamModule
}

func newApp() *App {
	app := &App{}
	// Build a router
	app.Router = mux.NewRouter()
	app.Router.HandleFunc("/motion", app.PostHandlerMotion).Methods(http.MethodPost)
	http.Handle("/", app.Router)
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
	}
	return app
}

func (a *App) sendErr(w http.ResponseWriter, msg string, code int) {
	response, _ := json.Marshal(msg)
	w.WriteHeader(code)
	w.Write(response)
	return
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

func (a *App) PostHandlerMotion(w http.ResponseWriter, r *http.Request) {
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
	// Take a snapshot
	data, err := a.Snapshots.Capture()
	if err != nil {
		zap.S().Error(err)
		a.sendErr(w, "", http.StatusInternalServerError)
		return
	}
	err = a.Snapshots.Send(data)
	if err != nil {
		zap.S().Error(err)
		a.sendErr(w, "", http.StatusInternalServerError)
		return
	}
	/*
		zap.S().Debugf("Raw: %v", string(raw))
		err = json.Unmarshal(raw, &s)
		if err != nil {
			zap.S().Error(err)
			a.sendErr(w, "Bad Request", http.StatusBadRequest)
			return
		}
	*/
}

func (a *App) PeriodicSnapshotHandler() {
}

func (a *App) Serve() {
	uri := fmt.Sprintf("%s:%s", a.Cfg.General.Server, a.Cfg.General.Port)
	zap.S().Infof("Listening on %s", uri)
	log.Fatal(http.ListenAndServe(uri, a.Router))
}
