package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/handlers"
	"github.com/kakaluote000/demo-api/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestRegisterHandler(t *testing.T) {
	app := app.NewApp()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/register", handlers.RegisterHandler(app))

	tests := []struct {
		name       string
		user       models.User
		wantStatus int
	}{
		{
			name: "Valid registration",
			user: models.User{
				Username: "testuser",
				Password: "password123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Invalid registration - empty username",
			user: models.User{
				Password: "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userData, _ := json.Marshal(tt.user)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(userData))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// 更多测试用例...