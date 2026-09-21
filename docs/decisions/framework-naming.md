---
id: framework-naming
title: 框架命名 Nib
type: decision
status: accepted
tags: [branding]
related: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 框架命名 Nib

## 决定
命名 Nib（笔尖）。CLI：nib new/dev/build/doctor/ship/audit。

## 理由
- 故事贴合定位：AI 时代代码由"笔"写成，框架是笔尖——小、准、快
- CLI 3 字母，高频命令体验好
- 2026-09-21 冲突检查：nib.dev 域名可注册；npm 名被占（Stylus 死库 nib）→ 用 @nibjs/* scope；Go module 用 nib.dev/*

## 待办
注册 nib.dev 域名（未注册前 Go module 路径已可正常使用）。
