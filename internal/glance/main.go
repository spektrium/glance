package glance

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var buildVersion = "dev"

func Main() int {
	options, err := parseCliOptions()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	switch options.intent {
	case cliIntentVersionPrint:
		fmt.Println(buildVersion)
	case cliIntentServe:
		// remove in v0.10.0
		if serveUpdateNoticeIfConfigLocationNotMigrated(options.configPath) {
			return 1
		}

		if err := serveApp(options.configPath); err != nil {
			fmt.Println(err)
			return 1
		}
	case cliIntentConfigValidate:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("Could not parse config file: %v\n", err)
			return 1
		}

		if _, err := newConfigFromYAML(contents); err != nil {
			fmt.Printf("Config file is invalid: %v\n", err)
			return 1
		}
	case cliIntentConfigPrint:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("Could not parse config file: %v\n", err)
			return 1
		}

		fmt.Println(string(contents))
	case cliIntentSensorsPrint:
		return cliSensorsPrint()
	case cliIntentMountpointInfo:
		return cliMountpointInfo(options.args[1])
	case cliIntentDiagnose:
		runDiagnostic()
	case cliIntentSecretMake:
		key, err := makeAuthSecretKey(AUTH_SECRET_KEY_LENGTH)
		if err != nil {
			fmt.Printf("Failed to make secret key: %v\n", err)
			return 1
		}

		fmt.Println(key)
	case cliIntentPasswordHash:
		password := options.args[1]

		if password == "" {
			fmt.Println("Password cannot be empty")
			return 1
		}

		if len(password) < 6 {
			fmt.Println("Password must be at least 6 characters long")
			return 1
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Failed to hash password: %v\n", err)
			return 1
		}

		fmt.Println(string(hashedPassword))
	}

	return 0
}

func serveApp(configPath string) error {
	// Keep a single HTTP server alive and swap the application on config changes so
	// visual editing and the admin panel can persist YAML without dropping connections.
	holder := &appHolder{configPath: configPath}
	exitChannel := make(chan struct{})
	hadValidConfigOnStartup := false
	var httpServer *http.Server

	onChange := func(newContents []byte) {
		if holder.consumeSkip() {
			return
		}

		if httpServer != nil {
			log.Println("Config file changed, reloading...")
		}

		app, err := holder.loadFromYAML(newContents)
		if err != nil {
			log.Printf("Config has errors: %v", err)

			if !hadValidConfigOnStartup {
				close(exitChannel)
			}

			return
		}

		holder.swap(app, app.routes())

		if hadValidConfigOnStartup {
			return
		}

		hadValidConfigOnStartup = true
		addr := fmt.Sprintf("%s:%d", app.Config.Server.Host, app.Config.Server.Port)
		absAssetsPath := ""
		if app.Config.Server.AssetsPath != "" {
			absAssetsPath, _ = filepath.Abs(app.Config.Server.AssetsPath)
		}

		httpServer = &http.Server{
			Addr:              addr,
			Handler:           holder,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
		}

		go func() {
			log.Printf("Starting server on %s (base-url: \"%s\", assets-path: \"%s\")\n",
				addr,
				app.Config.Server.BaseURL,
				absAssetsPath,
			)

			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Failed to start server: %v", err)
				close(exitChannel)
			}
		}()
	}

	onErr := func(err error) {
		log.Printf("Error watching config files: %v", err)
	}

	configContents, configIncludes, err := parseYAMLIncludes(configPath)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	stopWatching, err := configFilesWatcher(configPath, configContents, configIncludes, onChange, onErr)
	if err == nil {
		defer stopWatching()
	} else {
		log.Printf("Error starting file watcher, config file changes will require a manual restart. (%v)", err)
		onChange(configContents)
	}

	<-exitChannel
	return nil
}

func serveUpdateNoticeIfConfigLocationNotMigrated(configPath string) bool {
	if !isRunningInsideDockerContainer() {
		return false
	}

	if _, err := os.Stat(configPath); err == nil {
		return false
	}

	// glance.yml wasn't mounted to begin with or was incorrectly mounted as a directory
	if stat, err := os.Stat("glance.yml"); err != nil || stat.IsDir() {
		return false
	}

	templateFile, _ := templateFS.Open("v0.7-update-notice-page.html")
	bodyContents, _ := io.ReadAll(templateFile)

	fmt.Println("!!! WARNING !!!")
	fmt.Println("The default location of glance.yml in the Docker image has changed starting from v0.7.0.")
	fmt.Println("Please see https://github.com/glanceapp/glance/blob/main/docs/v0.7.0-upgrade.md for more information.")

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(bodyContents))
	})

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	server.ListenAndServe()

	return true
}
