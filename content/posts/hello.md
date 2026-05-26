---
title: Hello, ailog
date: 2026-05-26
tags:
  - meta
  - hello
description: 第一篇文章 — 驗證主題、frontmatter、code block、圖片渲染。
draft: false
---

歡迎來到 **ailog**。這是一個用 Go 寫的輕量 blog 系統，靈感來自 Hugo，但多了一個本地 web 編輯介面。

## 為什麼有這個東西

我之前一直用 Hugo，但每次想記點東西都要打開編輯器寫 markdown、跑 `hugo new`、commit、push…太繁瑣了。
這套東西想做到：

1. **像 Hugo 一樣輕量** — 純靜態輸出，可以直接放 GitHub Pages。
2. **有 web UI 可以編輯** — `ailog admin` 開啟本地 server，瀏覽器寫文章。
3. **權限管理** — 用 GitHub OAuth 驗證，只有自己能登入。
4. **一鍵部署** — push 後 GitHub Actions 自動 build、deploy。

## 一個 code block 範例

```go
package main

import "fmt"

func main() {
    fmt.Println("hello, ailog")
}
```

## 一段引用

> The best way to predict the future is to invent it.
> — Alan Kay

## 一個列表

- 不用寫 markdown 也能發文
- 不用 Node、不用 npm
- 單一 Go binary、零依賴

下一篇開始就是真的內容了。
