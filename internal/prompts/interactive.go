package prompts

import (
	"fmt"
	"strings"
)

type InteractiveStorySystemInstructionInput struct {
	CreatorPrompt    string
	Workspace        string
	ReplyTargetChars int
}

type InteractiveStoryPromptInput struct {
	Title             string
	Origin            string
	StoryTellerID     string
	StoryTeller       string
	BranchID          string
	ReplyTargetChars  int
	Characters        string
	WorldBuilding     string
	SnapshotStateJSON string
}

const defaultInteractiveReplyTargetChars = 1200

func BuildInteractiveStorySystemInstruction(in InteractiveStorySystemInstructionInput) string {
	var sb strings.Builder
	if creator := strings.TrimSpace(in.CreatorPrompt); creator != "" {
		sb.WriteString("# 创作者指令（最高优先级）\n\n")
		sb.WriteString(creator)
		sb.WriteString("\n\n---\n\n")
	}
	sb.WriteString("你是 Nova 的互动故事模式 Agent，只负责根据用户行动生成故事舞台上的下一回合内容。\n\n")
	sb.WriteString("## 模式边界\n")
	sb.WriteString("- 当前模式是互动故事模式，不是 IDE 写章节模式。\n")
	sb.WriteString("- 你的输出会流式展示到主屏幕的故事舞台，并由后端写入 interactive/story/story-{id}.jsonl。\n")
	sb.WriteString("- 禁止使用写文件工具，包括 write_file、edit_file、delete_file 以及任何会修改 workspace 文件的工具。\n")
	sb.WriteString("- 禁止调用 write_todos、任务计划工具或输出 <invoke> 工具调用片段；互动模式不维护待办列表。\n")
	sb.WriteString("- 不要创建或修改 chapters、outline、progress、characters 等文件；互动状态只能通过 <STATE_DELTA> JSON 表达。\n")
	sb.WriteString("- 可以基于已注入的故事上下文、共享设定和当前快照继续剧情。\n\n")
	sb.WriteString("## 互动叙事原则\n")
	sb.WriteString("- 这是文字小说 RPG 式体验：每一回合都要写出一段完整、有推进的小说叙事，而不是一句动作确认或系统裁定。\n")
	sb.WriteString("- 主角不是静止的摄像机。允许主角在本回合内观察、移动、试探、交谈、触碰物品、受到环境反馈，并和其他角色自然互动。\n")
	sb.WriteString("- 不要在主角每做一个小动作时立刻停下等待用户；只有当局势出现有意义的分岔、风险、代价或信息不足时，才把选择权交还给用户。\n")
	sb.WriteString("- 回合结尾要避免封闭式 ending；优先停在可行动的选择点、悬念点或决策点，让用户能继续决定主角怎么做。\n")
	sb.WriteString("- 可以在正文结尾自然呈现 2 到 4 个可选行动方向，但不要写成游戏 UI 菜单，也不要替用户决定下一步选择。\n\n")
	fmt.Fprintf(&sb, "- 本轮回复目标长度约为 %d 个中文字以内；你需要主动收束内容，优先写聚焦、有推进、可继续互动的一回合，不要依赖输出上限截断。\n\n", normalizeInteractiveReplyTargetChars(in.ReplyTargetChars))
	sb.WriteString("- 若本回合出现角色位置、状态、关系、在场人物、地点、时间或关键事件变化，必须在正文后追加完整 <STATE_DELTA> JSON；不要把状态变化只写在正文里。\n\n")
	sb.WriteString("## 输出协议\n")
	sb.WriteString("必须只输出以下结构，不要输出计划、解释、工具说明或 Markdown 标题：\n")
	sb.WriteString("<NARRATIVE>\n本回合展示在故事舞台上的正文\n</NARRATIVE>\n")
	sb.WriteString("<STATE_DELTA>\n{\"ops\":[{\"op\":\"set\",\"path\":\"on_stage\",\"value\":[\"角色名\"]}]}\n</STATE_DELTA>\n")
	sb.WriteString("如果没有明确状态变化，可以省略整个 <STATE_DELTA> 块。\n")
	if ws := strings.TrimSpace(in.Workspace); ws != "" {
		sb.WriteString("\n## 作品工作目录\n")
		sb.WriteString(ws)
		sb.WriteString("\n")
	}
	return sb.String()
}

func InteractiveStoryContext(in InteractiveStoryPromptInput) string {
	var sb strings.Builder
	sb.WriteString("[互动故事模式]\n")
	sb.WriteString("你正在为 Nova 的互动 story 子模式生成下一回合内容。输出会直接流式显示到故事舞台，并在结束后写入 interactive/story/story-{id}.jsonl。\n\n")
	sb.WriteString("## 输出协议\n")
	sb.WriteString("必须严格输出以下结构，不要输出额外解释、计划、工具说明或 Markdown 标题：\n")
	sb.WriteString("<NARRATIVE>\n")
	sb.WriteString("本回合面向读者展示的故事正文\n")
	sb.WriteString("</NARRATIVE>\n")
	sb.WriteString("<STATE_DELTA>\n")
	sb.WriteString("{\"ops\":[{\"op\":\"set\",\"path\":\"on_stage\",\"value\":[\"角色名\"]}]}\n")
	sb.WriteString("</STATE_DELTA>\n\n")
	sb.WriteString("如果本回合没有明确状态变化，可以省略整个 <STATE_DELTA> 块。STATE_DELTA 只记录本回合已经发生、确定成立的变化，不要记录未来计划。\n")
	sb.WriteString("状态 path 仅允许 on_stage、characters.<角色名>、events、location、time、pov 及其子路径；op 仅允许 set、merge、push、pull、inc、unset。\n\n")
	sb.WriteString("## 叙事节奏\n")
	sb.WriteString("- 本回合应该像小说片段一样推进场景：环境有反馈，人物有反应，对话和动作可以自然发生。\n")
	sb.WriteString("- 不要把用户输入只复述一遍后立刻停止；要写出行动带来的具体后果、发现、阻碍或机会。\n")
	sb.WriteString("- 不要替用户完成重大选择、不可逆决定或长期目标；这些应留到回合末尾成为下一步互动入口。\n")
	sb.WriteString("- 每回合结尾避免封闭收束；尽可能给用户留下清晰但开放的下一步选择。\n\n")
	fmt.Fprintf(&sb, "本轮 NARRATIVE 目标长度约为 %d 个中文字以内。请主动控制篇幅，保证结尾完整收束到开放选择点。\n\n", normalizeInteractiveReplyTargetChars(in.ReplyTargetChars))
	sb.WriteString("如果本回合改变了角色状态、在场角色、地点、时间或关键事件，请追加完整 STATE_DELTA。常用路径：on_stage、characters.<角色名>、events、location、time、pov。\n\n")
	sb.WriteString("## 故事信息\n")
	writeField(&sb, "标题", in.Title)
	writeField(&sb, "开端", in.Origin)
	writeField(&sb, "当前分支", in.BranchID)
	writeField(&sb, "讲述者 ID", in.StoryTellerID)
	writeBlock(&sb, "讲述者提示词", in.StoryTeller)
	writeBlock(&sb, "角色设定", in.Characters)
	writeBlock(&sb, "世界观设定", in.WorldBuilding)
	writeBlock(&sb, "当前互动状态快照(JSON)", in.SnapshotStateJSON)
	return sb.String()
}

func normalizeInteractiveReplyTargetChars(v int) int {
	if v <= 0 {
		return defaultInteractiveReplyTargetChars
	}
	return v
}

func InteractiveStoryTurnInstruction(message string) string {
	return fmt.Sprintf(`[互动输入]
用户本回合行动：
%s

请基于互动故事上下文续写下一回合。NARRATIVE 只写读者应看到的故事正文；STATE_DELTA 只写本回合造成的状态变化。
本回合要让主角作为故事人物正常与环境、物品和其他角色互动，写出行动带来的反馈和推进；不要每发生一个小动作就停下等待用户。
结尾请停在有意义的选择点、悬念点或决策点，让用户能决定下一步，但不要替用户做出重大选择。
必须使用 <NARRATIVE>...</NARRATIVE> 包裹正文；如有状态变化，再追加 <STATE_DELTA>...</STATE_DELTA> JSON。`, strings.TrimSpace(message))
}

func writeField(sb *strings.Builder, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "（空）"
	}
	fmt.Fprintf(sb, "- %s：%s\n", name, value)
}

func writeBlock(sb *strings.Builder, title, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "（空）"
	}
	fmt.Fprintf(sb, "\n## %s\n\n%s\n", title, value)
}
