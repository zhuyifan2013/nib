---
id: bridge-base64-response
title: 桥接响应经 base64 传递
type: decision
status: accepted
tags: [ipc, webview]
related: [error-envelope]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 桥接响应经 base64 传递

## 决定
Go → JS 的响应不直接内联 JSON，而是整体消息信封 base64 后由前端 atob + JSON.parse。

## 理由
JSON 内联进 JS 字符串需要多层转义（引号、反斜杠、控制字符、Unicode），手写转义极易出错；base64 无转义面，简单可靠。体积代价（~33%）对 IPC 消息可忽略。

## 教训（已踩坑）
前端桥必须解包信封：resolve(env.result) / reject(env.error)，不能把整条信封 resolve 给调用方。
