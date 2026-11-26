package repository

import (
	"testing"
)

func newImageFavoriteRepo(t *testing.T) *SQLiteRepository {
	t.Helper()
	t.Chdir(t.TempDir())
	repo, err := NewSQLiteRepository("test.db")
	if err != nil {
		t.Fatalf("创建测试仓库失败: %v", err)
	}
	t.Cleanup(func() { repo.DB().Close() })
	return repo
}

func sampleFavorite() *ImageFavoriteRecord {
	return &ImageFavoriteRecord{
		RepositoryName: "library/php",
		Registry:       "docker.io",
		PullDomain:     "docker.1ms.run",
		Tag:            "latest",
		Description:    "PHP 官方镜像",
		LogoURL:        "https://static.1ms.run/image/avatar/x",
		PullCount:      1417356357,
		StarCount:      7890,
		IsOfficial:     true,
	}
}

func TestImageFavoriteUpsertAndList(t *testing.T) {
	repo := newImageFavoriteRepo(t)

	stored, err := repo.UpsertImageFavorite(sampleFavorite())
	if err != nil {
		t.Fatalf("UpsertImageFavorite: %v", err)
	}
	if stored.ID == 0 {
		t.Fatal("应返回落库后的记录 ID")
	}
	if !stored.IsOfficial || stored.PullCount != 1417356357 {
		t.Fatalf("字段未正确落库: %+v", stored)
	}

	list, err := repo.ListImageFavorites()
	if err != nil {
		t.Fatalf("ListImageFavorites: %v", err)
	}
	if len(list) != 1 || list[0].RepositoryName != "library/php" {
		t.Fatalf("list = %+v", list)
	}

	count, err := repo.CountImageFavorites()
	if err != nil || count != 1 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
}

func TestImageFavoriteUpsertIdempotent(t *testing.T) {
	repo := newImageFavoriteRepo(t)

	first, err := repo.UpsertImageFavorite(sampleFavorite())
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// 同 (repository_name, tag) 再写一次：更新字段而不是新增行
	dup := sampleFavorite()
	dup.Description = "更新后的描述"
	dup.StarCount = 9999
	second, err := repo.UpsertImageFavorite(dup)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("upsert 产生了新行: first=%d second=%d", first.ID, second.ID)
	}
	if second.Description != "更新后的描述" || second.StarCount != 9999 {
		t.Fatalf("字段未更新: %+v", second)
	}

	count, _ := repo.CountImageFavorites()
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	// (repository_name, tag) 不同则是另一条收藏
	other := sampleFavorite()
	other.Tag = "8.4"
	if _, err := repo.UpsertImageFavorite(other); err != nil {
		t.Fatalf("other tag upsert: %v", err)
	}
	count, _ = repo.CountImageFavorites()
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestImageFavoriteDelete(t *testing.T) {
	repo := newImageFavoriteRepo(t)

	stored, err := repo.UpsertImageFavorite(sampleFavorite())
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.DeleteImageFavorite(stored.ID); err != nil {
		t.Fatalf("DeleteImageFavorite: %v", err)
	}
	if err := repo.DeleteImageFavorite(stored.ID); err == nil {
		t.Fatal("删除不存在的收藏应返回错误")
	}
}

func TestImageFavoriteListOrder(t *testing.T) {
	repo := newImageFavoriteRepo(t)

	for _, name := range []string{"a/nginx", "b/redis", "c/mysql"} {
		rec := sampleFavorite()
		rec.RepositoryName = name
		if _, err := repo.UpsertImageFavorite(rec); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
	}

	list, err := repo.ListImageFavorites()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 || list[0].RepositoryName != "c/mysql" {
		t.Fatalf("应按 id DESC 排序，最新在前: %+v", list)
	}
}
