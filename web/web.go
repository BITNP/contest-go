package web

import "embed"

// FS 内嵌前端静态文件，运行时不依赖外部构建产物。
//
//go:embed *.html *.js *.css
var FS embed.FS
