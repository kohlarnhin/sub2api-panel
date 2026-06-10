package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/zhujiangyong/sub2api-panel/backend/internal/register"
)

type RegisterHandler struct {
	svc *register.Service
}

func NewRegisterHandler(svc *register.Service) *RegisterHandler {
	return &RegisterHandler{svc: svc}
}

func (h *RegisterHandler) Template(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"template": h.svc.Template()})
}

func (h *RegisterHandler) LoginUser(c *gin.Context) {
	var payload register.UserLoginRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dashboard, err := h.svc.LoginUser(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (h *RegisterHandler) UserDashboard(c *gin.Context) {
	userID, ok := bindInt64Param(c, "user_id")
	if !ok {
		return
	}
	dashboard, err := h.svc.UserDashboard(c.Request.Context(), userID)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (h *RegisterHandler) UpdateUser(c *gin.Context) {
	var payload register.UserUpdateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dashboard, err := h.svc.UpdateUser(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (h *RegisterHandler) SaveUserPageConfig(c *gin.Context) {
	var payload register.UserPageConfigRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dashboard, err := h.svc.SaveUserPageConfig(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (h *RegisterHandler) UserEmails(c *gin.Context) {
	userID, ok := bindInt64Param(c, "user_id")
	if !ok {
		return
	}
	page := bindIntQuery(c, "page", 1)
	pageSize := bindIntQuery(c, "page_size", 10)
	search := c.Query("q")
	result, err := h.svc.UserEmails(c.Request.Context(), userID, page, pageSize, search)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RegisterHandler) GenerateUserEmails(c *gin.Context) {
	var payload register.UserEmailGenerateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.GenerateUserEmails(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error(), "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RegisterHandler) UploadUserAccountSub2API(c *gin.Context) {
	var payload register.UserSub2APIUploadRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.UploadUserAccountSub2API(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RegisterHandler) StartUserRegister(c *gin.Context) {
	var payload register.UserRegisterStartRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dashboard, err := h.svc.StartUserRegister(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (h *RegisterHandler) StopUserRegister(c *gin.Context) {
	var payload register.UserStopRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dashboard, err := h.svc.StopUserRegister(c.Request.Context(), payload.UserID)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dashboard)
}

func (h *RegisterHandler) StartHeroSMS(c *gin.Context) {
	var payload register.StartRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	session, err := h.svc.StartHeroSMS(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *RegisterHandler) StartHeroSMSBatch(c *gin.Context) {
	var payload register.StartRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	batch, err := h.svc.StartHeroSMSBatch(c.Request.Context(), payload)
	if err != nil {
		c.JSON(statusForRegisterError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, batch)
}

func (h *RegisterHandler) GetSession(c *gin.Context) {
	session, err := h.svc.GetSession(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *RegisterHandler) StopSession(c *gin.Context) {
	var payload register.StopRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	session, err := h.svc.Stop(payload.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *RegisterHandler) GetBatch(c *gin.Context) {
	batch, err := h.svc.GetBatch(c.Param("batch_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, batch)
}

func (h *RegisterHandler) StopBatch(c *gin.Context) {
	var payload register.StopRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	batch, err := h.svc.StopBatch(payload.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, batch)
}

func statusForRegisterError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return http.StatusConflict
}

func bindInt64Param(c *gin.Context, key string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || value <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " 无效"})
		return 0, false
	}
	return value, true
}

func bindIntQuery(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
