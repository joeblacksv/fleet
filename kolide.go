package main

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	appName        = "kolide"
	appDescription = "osquery command and control"
	versionMajor   = 0
	versionMinor   = 1
	versionPatch   = 0
	version        = fmt.Sprintf("%d.%d.%d", versionMajor, versionMinor, versionPatch)
)

var (
	app = kingpin.New(appName, appDescription)

	configPath = app.Flag("config", "configuration file").
			Short('c').
			OverrideDefaultFromEnvar("KOLIDE_CONFIG_PATH").
			ExistingFile()

	debug = app.Flag("debug", "Enable debug mode.").
		OverrideDefaultFromEnvar("KOLIDE_DEBUG").
		Bool()

	logJson = app.Flag("log_format_json", "Log in JSON format.").
		OverrideDefaultFromEnvar("KOLIDE_LOG_FORMAT_JSON").
		Bool()

	prepareDB = app.Command("prepare-db", "Create database tables")
	serve     = app.Command("serve", "Run the Kolide server")
)

func init() {
	// set gin mode to release to silence some superfluous logging
	gin.SetMode(gin.ReleaseMode)

	// configure logging
	logrus.AddHook(logContextHook{})

	// populate the global config data structure with sane defaults
	setDefaultConfigValues()
}

// logContextHook is a logrus hook which is used to contextualize application
// logs to include data stuch as line numbers, file names, etc.
type logContextHook struct{}

// Levels defines which levels the logContextHook logrus hook should apply to
func (hook logContextHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire defines what the logContextHook should actually do when it is triggered
func (hook logContextHook) Fire(entry *logrus.Entry) error {
	if pc, file, line, ok := runtime.Caller(8); ok {
		funcName := runtime.FuncForPC(pc).Name()

		entry.Data["file"] = path.Base(file)
		entry.Data["func"] = path.Base(funcName)
		entry.Data["line"] = line
	}

	return nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	// configure flag parsing and parse flags
	app.Version(version)
	args, err := app.Parse(os.Args[1:])

	// configure the application based on the flags that have been set
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if *logJson {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// if config hasn't been defined and the example config exists relative to
	// the binary, it's likely that the tool is being ran right after building
	// from source so we auto-populate the example config path.
	if *configPath == "" {
		if _, err = os.Stat("./tools/example_config.json"); err == nil {
			*configPath = "./tools/example_config.json"
		}
	}

	// if the user has defined a config path OR the example config is found
	// relative to the binary, load config content from the file. any content
	// in the config file will overwrite the default values
	if *configPath != "" {
		err = loadConfig(*configPath)
		if err != nil {
			logrus.Fatalf("Error loading config: %s", err.Error())
		}
	}

	// route the executable based on the sub-command
	switch kingpin.MustParse(args, err) {
	case prepareDB.FullCommand():
		db, err := openDB(config.MySQL.Username, config.MySQL.Password, config.MySQL.Address, config.MySQL.Database)
		if err != nil {
			logrus.Fatalf("Error opening database: %s", err.Error())
		}
		dropTables(db)
		createTables(db)
	case serve.FullCommand():
		fmt.Printf("=> %s %s application starting on https://%s\n", app.Name, version, config.Server.Address)
		fmt.Println("=> Run `kolide help serve` for more startup options")
		fmt.Println("Use Ctrl-C to stop\n\n")
		CreateServer().RunTLS(
			config.Server.Address,
			config.Server.Cert,
			config.Server.Key)

	}
}