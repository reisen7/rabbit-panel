package router

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"rabbit-panel/repository"
)

// 镜像收藏字段的安全校验。这些字段会拼进用户复制执行的 docker pull 命令，
// 必须拒绝空白、分号、反引号、$() 等任何能注入 shell 的字符。
var (
	imageRepoNameRe = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	imageTagRe      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	imageHostRe     = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]{1,5})?$`)
)

const maxImageFavorites = 200

func isValidImageRepoName(s string) bool {
	return len(s) > 0 && len(s) <= 200 && imageRepoNameRe.MatchString(s)
}

func isValidImageTag(s string) bool {
	return imageTagRe.MatchString(s)
}

func isValidImageHost(s string) bool {
	return s == "" || (len(s) <= 100 && imageHostRe.MatchString(s))
}

func sanitizeImageLogoURL(s string) string {
	if len(s) > 300 || strings.IndexFunc(s, unicode.IsSpace) >= 0 {
		return ""
	}
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		return s
	}
	return ""
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

// sanitizeImageFavorite 校验并清洗收藏记录。返回错误表示字段非法应拒绝入库。
func sanitizeImageFavorite(rec *repository.ImageFavoriteRecord) error {
	rec.RepositoryName = strings.TrimSpace(rec.RepositoryName)
	if !isValidImageRepoName(rec.RepositoryName) {
		return errInvalidFavorite("镜像名称格式不正确")
	}
	rec.Tag = strings.TrimSpace(rec.Tag)
	if rec.Tag == "" {
		rec.Tag = "latest"
	}
	if !isValidImageTag(rec.Tag) {
		return errInvalidFavorite("标签格式不正确")
	}
	rec.Registry = strings.TrimSpace(rec.Registry)
	if !isValidImageHost(rec.Registry) {
		return errInvalidFavorite("registry 格式不正确")
	}
	rec.PullDomain = strings.TrimSpace(rec.PullDomain)
	if !isValidImageHost(rec.PullDomain) {
		return errInvalidFavorite("pull_domain 格式不正确")
	}
	rec.LogoURL = sanitizeImageLogoURL(strings.TrimSpace(rec.LogoURL))
	rec.Description = truncateRunes(strings.TrimSpace(rec.Description), 500)
	if rec.PullCount < 0 {
		rec.PullCount = 0
	}
	if rec.StarCount < 0 {
		rec.StarCount = 0
	}
	return nil
}

type errInvalidFavorite string

func (e errInvalidFavorite) Error() string { return string(e) }

// handleImageFavoritesList GET /api/image-favorites
func (r *Router) handleImageFavoritesList(c *gin.Context) {
	list, err := r.app.SQLiteRepo.ListImageFavorites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*repository.ImageFavoriteRecord{}
	}
	c.JSON(http.StatusOK, list)
}

// handleImageFavoriteUpsert POST /api/image-favorites
func (r *Router) handleImageFavoriteUpsert(c *gin.Context) {
	var rec repository.ImageFavoriteRecord
	if err := c.ShouldBindJSON(&rec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if err := sanitizeImageFavorite(&rec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	count, err := r.app.SQLiteRepo.CountImageFavorites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count >= maxImageFavorites {
		// 达到上限时只允许更新已存在的 (repository_name, tag)
		list, err := r.app.SQLiteRepo.ListImageFavorites()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		exists := false
		for _, f := range list {
			if f.RepositoryName == rec.RepositoryName && f.Tag == rec.Tag {
				exists = true
				break
			}
		}
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "收藏数量已达上限（200）"})
			return
		}
	}

	stored, err := r.app.SQLiteRepo.UpsertImageFavorite(&rec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stored)
}

// handleImageFavoriteDelete DELETE /api/image-favorites?id=
func (r *Router) handleImageFavoriteDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	if err := r.app.SQLiteRepo.DeleteImageFavorite(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
