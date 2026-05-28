# Ideas
## WIP
- 基础体验
  - 完善的 Skills 系统
  - /brainstorm /spec /plan /execute /review
  - 章节细纲，从大纲到->章节细纲->章节草稿->章节定稿
- Agent能力
  - 支持完善的上下文管理，memory
  - 考虑：自定义Agent
- ide 日志
- Tantivy / MeiliSearch 全局搜索
- 重构：考虑 Code Mirror6 编辑器，实验效果
- 调试模式，开启后可以看到context组成

## 互动模式
- story teller 可以按自己风格规划暗线剧情和事件，根据用户行为动态调整，保证用户有一个连续的互动体验
- 支持开局配置，配置多种开局可以选择
- 互动模式支持通过 agent+skill 来初始化世界观&角色&开局
- 优化导入酒馆v2角色卡，优化内容和世界观

我现在左侧是通过markdown来管理世界观，角色和创作者指令，感觉展示不太友好，编辑也不方便，且我希望能以不同 story teller 为核心，支持更灵活的prompt配置，比如
  - 有些prompt可以每轮插入到thinking中，增强agent follow instruction 的能力
  - 不同的story teller 可以分别配置或自定义增加配置
  - 支持通过 agent 来进行编辑修改
  - 允许修改存储数据结构，先讨论一下



我倾向的实施顺序
第一阶段：先做后端数据模型和 prompt slot 编排，不大改 UI。 --done
第二阶段：把 teller 从 Markdown frontmatter 升级为 JSON 配置，支持多个 prompt slot。
第三阶段：左侧资料库改成角色卡/世界观卡片/创作规则面板。
第四阶段：加资料编辑 Agent，而且应该支持下基本的版本管理，不用git，自己实现。
第五阶段：迁移 README、CHANGELOG、测试，并保持旧 Markdown 兼容读取。


## NEED FIX
- state agent 应该输出快一点，不要thinking了，直接输出


# 规划
- 多语言支持
- 互动创作模式
- 剧情分支系统，允许从特定节点开始，分出不同的剧情线延续，允许对比不同的分支然后选择一个合并
- 版本管理：不用git，自己实现
- 支持导入小说

- prompt 高级自定义
- 支持在diff view中点击accept/reject按钮，确认或拒绝当前diff
