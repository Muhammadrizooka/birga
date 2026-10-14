package ginlog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogExceptLogsJSONResponse(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LogExcept(nil), Recovery(false, func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error_code": -50})
	}))
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"data": strings.Repeat("x", MaxLoggedBody)}) })
	r.GET("/panic", func(*gin.Context) { panic("boom") })

	for _, path := range []string{"/ok", "/panic"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	}

	entries := logs.FilterMessage("request").All()
	if len(entries) != 2 {
		t.Fatalf("got %d request lines", len(entries))
	}

	if got := entries[0].ContextMap()["response"].(string); !strings.HasSuffix(got, "...(cut)") || len(got) != MaxLoggedBody+len("...(cut)") {
		t.Fatalf("long body not cut: %d bytes", len(got))
	}

	if got := entries[1].ContextMap()["response"]; got != `{"error_code":-50}` {
		t.Fatalf("panic response %v", got)
	}
}
