package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

//go:embed all:html
var webRoot embed.FS

func RegisterUI(engine *gin.Engine, devMode bool, staticHosting bool, uiProxyUrl string) error {
	if staticHosting {
		log.Info().Msg("Hosting embedded static files for UI")
		if !devMode {
			log.Info().Msg("Setting up cache control middleware")
			engine.Use(CacheControlMiddleware)
		}
		engine.Use(GetEmbeddedFileHandler())
	} else if uiProxyUrl != "" {
		log.Info().Msg("Setting up reverse proxy for UI")
		engine.Use(GetProxyHandler(uiProxyUrl))
	}
	return nil
}

func GetEmbeddedFileHandler() gin.HandlerFunc {
	sub, err := fs.Sub(webRoot, "html")
	if err != nil {
		log.Panic().Err(err).Msg("error getting subdirectory for webRoot")
	}

	readFile := func(path string) ([]byte, error) {
		path = strings.TrimPrefix(path, "/")
		file, err := sub.Open(path)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	indexFileData, err := readFile("index.html")
	if err != nil {
		log.Panic().Err(err).Msg("error reading index.html")
	}

	return gin.HandlerFunc(func(c *gin.Context) {
		// Skip API, websocket, and auth routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/ws") || strings.HasPrefix(c.Request.URL.Path, "/auth/") {
			c.Next()
			return
		}

		_, err := readFile(c.Request.URL.Path)
		if err != nil {
			log.Trace().Msgf("'%s' not found as file, serving '/index.html", c.Request.URL.Path)
			c.Data(http.StatusOK, "text/html", indexFileData)
			c.Done()
			return
		}

		http.FileServer(http.FS(sub)).ServeHTTP(c.Writer, c.Request)
		c.Done()
	})
}

func GetProxyHandler(uiProxyUrl string) gin.HandlerFunc {
	proxyUrl, err := url.Parse(uiProxyUrl)
	if err != nil {
		log.Panic().Err(err).Msgf("unable to parse target url '%s'", uiProxyUrl)
	}
	proxy := httputil.NewSingleHostReverseProxy(proxyUrl)

	// Remove Content-Length to avoid mismatch issues with Gin's response writer.
	// This forces chunked transfer encoding instead.
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Content-Length")
		return nil
	}

	// Custom error handler to prevent Gin from writing error responses
	// after the proxy has already started writing
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Debug().Err(err).Str("path", req.URL.Path).Msg("reverse proxy error")
	}

	return func(c *gin.Context) {
		// Skip API, websocket, and auth routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/ws") || strings.HasPrefix(c.Request.URL.Path, "/auth/") {
			c.Next()
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func CacheControlMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Next()
}
