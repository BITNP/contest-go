package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"math/rand/v2"
	"mime"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"contest-go/web"
)

// pageFiles 是所有服务端渲染页面，启动时各解析为独立模板。
var pageFiles = []string{"index.html", "contest.html", "info.html"}

type pageTemplates map[string]*template.Template

// parsePageTemplates 启动时解析全部页面模板；每个页面与 layout 组成独立模板集，
// 避免不同页面同名的 content/scripts 定义互相覆盖。
func parsePageTemplates() (pageTemplates, error) {
	templates := make(pageTemplates, len(pageFiles))
	for _, page := range pageFiles {
		t, err := template.ParseFS(web.FS, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("解析模板 %s: %w", page, err)
		}
		templates[page] = t
	}
	return templates, nil
}

// navItem 是导航条链接，Active 由当前路径决定，class 在 Go 侧拼好，
// 模板里不再重复条件拼接。
type navItem struct {
	Href   string
	Label  string
	Active bool
}

func (n navItem) DesktopClass() string {
	return navClass(n.Active, "rounded-md px-3 py-2 text-sm font-medium")
}

func (n navItem) MobileClass() string {
	return navClass(n.Active, "block rounded-md px-3 py-2 text-base font-medium")
}

func navClass(active bool, base string) string {
	if active {
		return "bg-gray-900 text-white " + base
	}
	return "text-gray-300 hover:bg-gray-700 hover:text-white " + base
}

type pageData struct {
	DevLogin      bool
	Authenticated bool
	Username      string
	Title         string
	Path          string
	ShowHeading   bool
	NavItems      []navItem
	NQuestions    int
	TotalScore    int
	MaxTries      int
	Year          int
	BackgroundURL string
	DeadlineText  string
}

func (s *Server) pageHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, ok := s.templates[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "layout", s.pageData(r, name)); err != nil {
			http.Error(w, "页面渲染失败", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	}
}

func (s *Server) pageData(r *http.Request, name string) pageData {
	username, authenticated := s.Session.Username(r)

	data := pageData{
		DevLogin:      s.DevLogin,
		Authenticated: authenticated,
		Username:      username,
		Path:          r.URL.Path,
		Year:          time.Now().Year(),
		BackgroundURL: s.randomBackground(name),
		NavItems:      buildNavItems(r.URL.Path, authenticated),
	}
	switch name {
	case "index.html":
		data.Title = "主页"
	case "contest.html":
		data.Title = "答题"
		data.ShowHeading = true
	case "info.html":
		data.Title = "历史成绩"
		data.ShowHeading = true
	default:
		data.Title = "国防知识竞赛"
	}

	if s.Exam != nil {
		data.MaxTries = s.Exam.Config.MaxTries
		for cat, count := range s.Exam.Config.PaperCounts {
			data.NQuestions += count
			data.TotalScore += count * s.Exam.Config.ScorePerQuestion[cat]
		}
		data.DeadlineText = formatDuration(s.Exam.Config.Deadline)
	}
	return data
}

func buildNavItems(path string, authenticated bool) []navItem {
	items := []navItem{{Href: "/", Label: "主页", Active: path == "/"}}
	if authenticated {
		items = append(items,
			navItem{Href: "/contest", Label: "答题", Active: path == "/contest"},
			navItem{Href: "/info", Label: "历史成绩", Active: path == "/info"},
		)
	}
	return items
}

func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds > 0 && seconds%60 == 0 {
		return fmt.Sprintf("%d分钟", seconds/60)
	}
	return fmt.Sprintf("%d秒", seconds)
}

// staticAsset 是启动时索引的静态资源：内容、类型与 ETag。
type staticAsset struct {
	data        []byte
	contentType string
	etag        string
}

type staticAssets map[string]*staticAsset

// staticContentTypes 显式声明常见扩展名，避免依赖容器里可能缺失的 mime 数据库。
var staticContentTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".json":  "application/json",
	".map":   "application/json",
	".woff2": "font/woff2",
}

func loadStaticAssets() (staticAssets, error) {
	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, fmt.Errorf("加载静态资源: %w", err)
	}
	assets := staticAssets{}
	err = fs.WalkDir(staticFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(staticFS, p)
		if err != nil {
			return err
		}
		ext := strings.ToLower(path.Ext(p))
		ct := staticContentTypes[ext]
		if ct == "" {
			ct = mime.TypeByExtension(ext)
		}
		if ct == "" {
			ct = "application/octet-stream"
		}
		sum := sha256.Sum256(data)
		assets[p] = &staticAsset{
			data:        data,
			contentType: ct,
			etag:        `"` + hex.EncodeToString(sum[:8]) + `"`,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("加载静态资源: %w", err)
	}
	return assets, nil
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/static/")
	asset, ok := s.static[name]
	if name == "" || strings.Contains(name, "..") || !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", asset.contentType)
	w.Header().Set("ETag", asset.etag)
	// no-cache：每次都协商，命中 ETag 则 304，保证发布新版本后立即可见。
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == asset.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(asset.data)
}

// randomBackground 为答题页/成绩页随机挑选一张背景图；无可用背景时返回空串。
func (s *Server) randomBackground(page string) string {
	if page != "contest.html" && page != "info.html" {
		return ""
	}
	if len(s.backgrounds) == 0 {
		return ""
	}
	return "/static/" + s.backgrounds[rand.IntN(len(s.backgrounds))]
}

// collectBackgrounds 从静态资源索引中收集 img/background/ 下的文件，
// 增删背景图不需要改代码。
func collectBackgrounds(assets staticAssets) []string {
	var out []string
	for name := range assets {
		if strings.HasPrefix(name, "img/background/") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
