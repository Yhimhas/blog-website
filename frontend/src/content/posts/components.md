---
title: "组件之间，应该如何好好沟通？"
slug: components
published: "2026-09-14"
category: 开发笔记
summary: "用 props 描述输入，用 emit 表达变化。整理一次关于 Vue 数据流的小练习。"
tags: ["TypeScript","组件设计"]
---

## 明确输入

props 是组件与外部约定的输入。用 TypeScript 写清楚类型，可以在接入组件的时候尽早发现遗漏。比起把整个页面状态都传进去，明确传入所需字段更容易理解。

## 明确变化

子组件发出事件，父组件决定如何更新状态。比如展开简介时发送 update:visible，配合 v-model 使用，就能让显示状态有一个清楚的来源。
