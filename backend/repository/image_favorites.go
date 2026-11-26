package repository

import (
	"fmt"
	"time"
)

// ImageFavoriteRecord 收藏的镜像条目。字段与镜像搜索结果一致，
// pull_domain/registry/tag 由路由层校验后才会落库。
type ImageFavoriteRecord struct {
	ID             int64     `json:"id"`
	RepositoryName string    `json:"repository_name"`
	Registry       string    `json:"registry"`
	PullDomain     string    `json:"pull_domain"`
	Tag            string    `json:"tag"`
	Description    string    `json:"description"`
	LogoURL        string    `json:"logo_url"`
	PullCount      int64     `json:"pull_count"`
	StarCount      int64     `json:"star_count"`
	IsOfficial     bool      `json:"is_official"`
	CreatedAt      time.Time `json:"created_at"`
}

// IImageFavoriteRepository 镜像收藏数据访问接口
type IImageFavoriteRepository interface {
	ListImageFavorites() ([]*ImageFavoriteRecord, error)
	UpsertImageFavorite(rec *ImageFavoriteRecord) (*ImageFavoriteRecord, error)
	DeleteImageFavorite(id int64) error
	CountImageFavorites() (int, error)
}

const imageFavoriteColumns = `id, repository_name, registry, pull_domain, tag, description, logo_url, pull_count, star_count, is_official, created_at`

func scanImageFavorite(row interface{ Scan(...any) error }) (*ImageFavoriteRecord, error) {
	rec := &ImageFavoriteRecord{}
	var official int
	err := row.Scan(&rec.ID, &rec.RepositoryName, &rec.Registry, &rec.PullDomain, &rec.Tag,
		&rec.Description, &rec.LogoURL, &rec.PullCount, &rec.StarCount, &official, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	rec.IsOfficial = official != 0
	return rec, nil
}

// ListImageFavorites 列出全部收藏，最新在前
func (r *SQLiteRepository) ListImageFavorites() ([]*ImageFavoriteRecord, error) {
	rows, err := r.db.Query(`SELECT ` + imageFavoriteColumns + ` FROM image_favorites ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询镜像收藏失败: %w", err)
	}
	defer rows.Close()

	list := make([]*ImageFavoriteRecord, 0)
	for rows.Next() {
		rec, err := scanImageFavorite(rows)
		if err != nil {
			return nil, fmt.Errorf("读取镜像收藏失败: %w", err)
		}
		list = append(list, rec)
	}
	return list, rows.Err()
}

// UpsertImageFavorite 按 (repository_name, tag) 唯一键写入或更新收藏，返回落库后的记录
func (r *SQLiteRepository) UpsertImageFavorite(rec *ImageFavoriteRecord) (*ImageFavoriteRecord, error) {
	official := 0
	if rec.IsOfficial {
		official = 1
	}
	_, err := r.db.Exec(
		`INSERT INTO image_favorites
			(repository_name, registry, pull_domain, tag, description, logo_url, pull_count, star_count, is_official)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(repository_name, tag) DO UPDATE SET
			registry = excluded.registry,
			pull_domain = excluded.pull_domain,
			description = excluded.description,
			logo_url = excluded.logo_url,
			pull_count = excluded.pull_count,
			star_count = excluded.star_count,
			is_official = excluded.is_official`,
		rec.RepositoryName, rec.Registry, rec.PullDomain, rec.Tag,
		rec.Description, rec.LogoURL, rec.PullCount, rec.StarCount, official)
	if err != nil {
		return nil, fmt.Errorf("保存镜像收藏失败: %w", err)
	}

	stored, err := scanImageFavorite(r.db.QueryRow(
		`SELECT `+imageFavoriteColumns+` FROM image_favorites WHERE repository_name = ? AND tag = ?`,
		rec.RepositoryName, rec.Tag))
	if err != nil {
		return nil, fmt.Errorf("读取镜像收藏失败: %w", err)
	}
	return stored, nil
}

// DeleteImageFavorite 删除一条收藏
func (r *SQLiteRepository) DeleteImageFavorite(id int64) error {
	result, err := r.db.Exec(`DELETE FROM image_favorites WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除镜像收藏失败: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("镜像收藏不存在")
	}
	return nil
}

// CountImageFavorites 返回收藏总数（用于上限校验）
func (r *SQLiteRepository) CountImageFavorites() (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM image_favorites`).Scan(&count); err != nil {
		return 0, fmt.Errorf("统计镜像收藏失败: %w", err)
	}
	return count, nil
}
