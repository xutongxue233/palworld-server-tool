package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/api"
	"github.com/zaigie/palworld-server-tool/docs"
	"github.com/zaigie/palworld-server-tool/internal/config"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/discovery"
	"github.com/zaigie/palworld-server-tool/internal/logger"
	"github.com/zaigie/palworld-server-tool/internal/system"
	"github.com/zaigie/palworld-server-tool/internal/task"
)

var (
	version = "Develop"
	conf    config.Config
)

//	@SecurityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name							Authorization

//	@SecurityDefinitions.apikey	FleetNodeAuth
//	@in							header
//	@name							X-PST-Fleet-Token

// @license.name	Apache 2.0
// @license.url	http://www.apache.org/licenses/LICENSE-2.0.html
func main() {
	db := database.GetDB()
	defer db.Close()

	configResult := config.Init(db, &conf)
	if configResult.MigratedFrom != "" {
		logger.Infof("Imported legacy configuration from %s into pst.db; the YAML file is no longer read.\n", configResult.MigratedFrom)
	}
	discoveryStatus := discovery.Initialize()
	if discoveryStatus.AutoConfigured {
		logger.Infof("Automatically configured PalServer from candidate %s\n", discoveryStatus.SelectedCandidateID)
	}
	for _, warning := range discoveryStatus.Warnings {
		logger.Warnf("PalServer discovery: %s\n", warning)
	}
	config.FreezeRuntime()

	docs.SwaggerInfo.Title = "Palworld Manage API"
	docs.SwaggerInfo.Version = version
	docs.SwaggerInfo.Host = fmt.Sprintf("127.0.0.1:%d", viper.GetInt("web.port"))
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http"}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("version", version)
		c.Next()
	})
	api.RegisterRouter(router)

	assetsFS, _ := fs.Sub(assets, assetsRoot)
	router.StaticFS("/assets", http.FS(assetsFS))
	mapTilesFS, _ := fs.Sub(mapTiles, mapRoot)
	router.StaticFS("/map/tiles", http.FS(mapTilesFS))
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/x-icon", favicon)
	})
	router.GET("/", func(c *gin.Context) {
		file, err := indexHTML.ReadFile(indexHTMLPath)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", file)
	})
	router.GET("/pal-conf", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/#/configuration")
	})
	router.GET("/pal-conf/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/#/configuration")
	})

	localIP, err := system.GetLocalIP()
	if err != nil {
		logger.Errorf("%v\n", err)
	}
	logger.Info("Starting PalWorld Server Tool web service...\n")
	logger.Infof("Version: %s\n", version)
	logger.Infof("Configuration storage: %s\n", config.StorageName())
	logger.Infof("Listening on http://127.0.0.1:%d or http://%s:%d\n", viper.GetInt("web.port"), localIP, viper.GetInt("web.port"))
	logger.Infof("Swagger on http://127.0.0.1:%d/swagger/index.html\n", viper.GetInt("web.port"))

	task.Schedule(db)
	defer task.Shutdown()

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", viper.GetInt("web.port")),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	serverDone := make(chan error, 1)
	go func() {
		if viper.GetBool("web.tls") {
			serverDone <- server.ListenAndServeTLS(viper.GetString("web.cert_path"), viper.GetString("web.key_path"))
			return
		}
		serverDone <- server.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigChan:
		logger.Info("Server gracefully stopped\n")
	case serveErr := <-serverDone:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Errorf("Server exited with error: %v\n", serveErr)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = server.Shutdown(ctx)
	cancel()
}
