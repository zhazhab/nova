---
name: continue
description: 续写小说章节。当用户要求写下一章、继续写、开始写某一章时使用。
---

# 章节续写

根据大纲、角色状态和前文内容续写小说章节。

## 工作流程

1. 使用 read_file 读取 `setting/outline.md` 确认当前要写的章节方向
2. 使用 read_file 读取 `setting/characters.md` 获取角色状态
3. 使用 read_file 读取 `setting/progress.md` 获取最近章节摘要
4. 必须读取前面至少 2 章正文，确保本章与前文自然衔接
5. 根据大纲方向、角色状态和前两章正文，创作本章内容
6. 使用 write_file 将章节写入 `chapters/chXX-章节名.md`
7. 不更改 `setting/outline.md`
8. 使用 write_file/update_file 更新 `setting/progress.md`（当前进度 + 本章摘要）
9. 使用 read_file + write_file/update_file 更新 `setting/characters.md`（角色状态变化）

## 写作要求

- 严格遵循大纲的章节走向和摘要
- 大纲只作为长期结构和章节方向参考，续写完成后不要修改大纲文件
- 写作推进主要通过 `setting/progress.md` 追踪，记录当前进度、已完成章节摘要和短期衔接提示
- 角色变化只写入 `setting/characters.md`，记录角色状态、关系、能力、伤势、心理和位置变化，不写章节规划
- 保持角色性格和说话方式一致
- 与前面至少两章自然衔接（注意情节、时间、地点、人物状态的连贯）
- 注意伏笔的埋设和呼应
- 遵循指定的字数要求
- 章节正文使用纯文本自然段，禁止使用 Markdown 标题、列表、引用、加粗等标记
- 章节开头直接开始叙事，不要输出章节标题

## 章节文件格式

正文内容直接使用自然段，段落间空行分隔。

## 进度文件格式

# 创作进度

- 当前进度：第X章已完成
- 总章节数：Y 章
- 已完成：Z 章

## 最近章节摘要

### 第X章：章节标题
摘要内容（200字以内，概括主要情节和角色变化）
