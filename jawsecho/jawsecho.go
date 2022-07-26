package jawsecho

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/linkdata/jaws"
)

// Setup adds JaWS handlers to a given Echo v4 router and if jw.Logger is nil
// it is set to router.StdLogger.
func Setup(router *echo.Echo, jw *jaws.Jaws) {
	if jw.Logger == nil {
		jw.Logger = router.StdLogger
	}
	router.GET(jaws.JavascriptPath(), func(c echo.Context) error {
		hdr := c.Response().Header()
		hdr.Set(echo.HeaderCacheControl, "public, max-age=31536000, s-maxage=31536000, immutable")
		hdr.Set(echo.HeaderVary, echo.HeaderAcceptEncoding)
		js := jaws.JavascriptText()
		if strings.Contains(c.Request().Header.Get(echo.HeaderAcceptEncoding), "gzip") {
			js = jaws.JavascriptGZip()
			hdr.Set(echo.HeaderContentEncoding, "gzip")
		}
		return c.Blob(http.StatusOK, "application/javascript; charset=utf-8", js)
	})
	router.GET("/jaws/ping", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		select {
		case <-jw.Done():
			return c.NoContent(http.StatusServiceUnavailable)
		default:
			return c.NoContent(http.StatusOK)
		}
	})
	router.GET("/jaws/:key", func(c echo.Context) error {
		if jawsKey, err := strconv.ParseInt(c.Param("key"), 16, 64); err == nil {
			if rq := jw.UseRequest(jawsKey, c.Request().RemoteAddr); rq != nil {
				rq.ServeHTTP(c.Response().Writer, c.Request())
				return nil
			}
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		return c.NoContent(http.StatusNotFound)
	})
}
