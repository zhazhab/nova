import { useState } from 'react'
import { BookOpen, Plus, Pencil, Trash2, Check, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { TooltipIconButton } from '@/components/common/tooltip-icon-button'
import {
  createBook,
  getBookInfo,
  removeBook,
  switchWorkspace,
  updateBookInfo,
  type BookMeta,
  type BookRecord,
} from '@/lib/api'

interface HomeViewProps {
  /** 当前工作区路径，用于高亮当前书籍并作为父目录推断默认值 */
  workspace: string
  /** 用户 Nova 数据目录，新建书籍默认创建在该目录下 */
  novaDir: string
  /** 最近书籍列表 */
  books: BookRecord[]
  /** 切换到指定 workspace 后由父组件刷新业务状态 */
  onSwitch: (path: string) => void
  /** 书籍记录有变更时通知父组件刷新列表 */
  onBooksChange: () => void
  /** 关闭全局书籍管理弹窗 */
  onClose?: () => void
}

/** 计算相对时间描述 */
function relativeTime(isoStr: string): string {
  if (!isoStr) return ''
  const diff = Date.now() - new Date(isoStr).getTime()
  if (diff < 0) return '刚刚'
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}天前`
  const months = Math.floor(days / 30)
  return `${months}月前`
}

const inputCls = 'w-full rounded border border-[#3a3d44] bg-[#25262a] px-2 py-1 text-xs text-[#d7dbe2] outline-none focus:border-[#4a4d54]'

/** 书籍管理视图：集中展示、创建、打开和编辑最近书籍。 */
export function HomeView({ workspace, novaDir, books, onSwitch, onBooksChange, onClose }: HomeViewProps) {
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [createTitle, setCreateTitle] = useState('')
  const [createAuthor, setCreateAuthor] = useState('')
  const [createDesc, setCreateDesc] = useState('')
  const [createError, setCreateError] = useState('')
  const [creating, setCreating] = useState(false)

  const [editingBookPath, setEditingBookPath] = useState<string | null>(null)
  const [editTitle, setEditTitle] = useState('')
  const [editAuthor, setEditAuthor] = useState('')
  const [editDesc, setEditDesc] = useState('')
  const [editLoading, setEditLoading] = useState(false)
  const [editSaving, setEditSaving] = useState(false)

  /** 打开新建书籍表单，新书统一创建在用户 Nova 数据目录下 */
  const openCreateForm = () => {
    setShowCreateForm(true)
    setCreateTitle('')
    setCreateAuthor('')
    setCreateDesc('')
    setCreateError('')
  }

  /** 提交新建书籍 */
  const handleCreate = async () => {
    if (!createTitle.trim()) { setCreateError('书名不能为空'); return }
    if (!novaDir.trim()) { setCreateError('Nova 数据目录未就绪，请稍后重试'); return }
    setCreating(true)
    setCreateError('')
    try {
      const data = await createBook(
        createTitle.trim(),
        createAuthor.trim() || undefined,
        createDesc.trim() || undefined,
      )
      onSwitch(data.workspace)
      setShowCreateForm(false)
      onBooksChange()
    } catch (e: unknown) {
      setCreateError(e instanceof Error ? e.message : '创建失败')
    } finally {
      setCreating(false)
    }
  }

  /** 切换到指定书籍 */
  const handleSwitch = async (path: string) => {
    try {
      const data = await switchWorkspace(path)
      onSwitch(data.workspace || path)
    } catch (e) {
      console.error('切换 workspace 失败', e)
    }
  }

  /** 移除最近书籍记录（不删除磁盘内容） */
  const handleRemove = async (path: string) => {
    try {
      await removeBook(path)
      onBooksChange()
    } catch (e) {
      console.error('移除书籍记录失败', e)
    }
  }

  /** 进入编辑模式，先拉取完整元信息 */
  const startEdit = async (book: BookRecord) => {
    setEditingBookPath(book.path)
    setEditTitle(book.name)
    setEditAuthor(book.author || '')
    setEditDesc('')
    setEditLoading(true)
    try {
      const meta: BookMeta = await getBookInfo(book.path)
      setEditTitle(meta.title)
      setEditAuthor(meta.author)
      setEditDesc(meta.description)
    } catch {
      // 拉取失败时回退使用列表里的基础信息
    } finally {
      setEditLoading(false)
    }
  }

  /** 保存书籍编辑 */
  const handleSaveEdit = async () => {
    if (!editingBookPath) return
    setEditSaving(true)
    try {
      await updateBookInfo(editingBookPath, editTitle.trim(), editAuthor.trim(), editDesc.trim())
      setEditingBookPath(null)
      onBooksChange()
    } catch (e) {
      console.error('保存书籍信息失败', e)
    } finally {
      setEditSaving(false)
    }
  }

  return (
    <div className="flex h-full min-w-0 flex-col bg-[#1b1c1f] text-[#d7dbe2]">
      <div className="flex h-9 shrink-0 items-center gap-2 border-b border-[#303238] bg-[#202124] px-4 text-xs">
        <BookOpen className="h-3.5 w-3.5 text-[#a8adb7]" />
        <span className="font-medium text-[#d7dbe2]">书籍管理</span>
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            className="ml-auto rounded p-1 text-[#858b96] hover:bg-[#303238] hover:text-[#d7dbe2]"
            aria-label="关闭书籍管理"
            title="关闭书籍管理"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      <ScrollArea className="flex-1">
        <div className="mx-auto flex max-w-3xl flex-col gap-4 px-6 py-6">
          {/* 当前书籍 */}
          <section className="rounded border border-[#303238] bg-[#202124] p-4">
            <div className="mb-2 text-xs font-medium text-[#c5c9d1]">当前书籍</div>
            <div className="text-sm text-[#d7dbe2]">{workspace || '未设置工作区'}</div>
          </section>

          {/* 书籍列表 */}
          <section className="rounded border border-[#303238] bg-[#202124] p-4">
            <div className="mb-3 flex items-center justify-between">
              <div className="text-xs font-medium text-[#c5c9d1]">最近书籍</div>
              {!showCreateForm && (
                <Button
                  type="button"
                  size="xs"
                  variant="ghost"
                  className="flex items-center gap-1 text-[#aeb4bf] hover:bg-[#303238]"
                  onClick={openCreateForm}
                >
                  <Plus className="h-3.5 w-3.5" />
                  新建书籍
                </Button>
              )}
            </div>

            {showCreateForm && (
              <div className="mb-4 space-y-2 rounded bg-[#25262a] p-3">
                <div className="text-xs font-medium text-[#c5c9d1]">新建书籍</div>
                <Input
                  type="text"
                  value={createTitle}
                  onChange={(e) => setCreateTitle(e.target.value)}
                  placeholder="书名（必填）"
                  className={inputCls}
                  autoFocus
                />
                <Input
                  type="text"
                  value={createAuthor}
                  onChange={(e) => setCreateAuthor(e.target.value)}
                  placeholder="作者（选填）"
                  className={inputCls}
                />
                <div className="rounded border border-[#303238] bg-[#1b1c1f] px-2 py-1.5 text-xs text-[#8f98a8]">
                  新书将创建在：<span className="text-[#c5c9d1]">{novaDir || 'Nova 数据目录加载中...'}</span>
                </div>
                <Textarea
                  value={createDesc}
                  onChange={(e) => setCreateDesc(e.target.value)}
                  placeholder="简介（选填）"
                  rows={3}
                  className={inputCls + ' min-h-0 resize-none'}
                />
                {createError && <div className="text-xs text-red-400">{createError}</div>}
                <div className="flex items-center justify-end gap-2">
                  <Button type="button" size="xs" variant="ghost" className="text-[#858b96] hover:bg-[#303238]" onClick={() => setShowCreateForm(false)}>取消</Button>
                  <Button type="button" size="xs" className="bg-[#4a4d54] text-white hover:bg-[#5a5d64]" disabled={creating || !novaDir.trim()} onClick={handleCreate}>
                    {creating ? '创建中...' : '创建'}
                  </Button>
                </div>
              </div>
            )}

            {books.length === 0 ? (
              <div className="py-6 text-center text-xs text-[#858b96]">暂无书籍记录</div>
            ) : (
              <div className="space-y-1">
                {books.map((book) => {
                  const isCurrent = book.path === workspace
                  const isEditing = editingBookPath === book.path

                  if (isEditing) {
                    return (
                      <div key={book.path} className="space-y-1.5 rounded bg-[#25262a] p-3">
                        {editLoading ? (
                          <div className="py-2 text-center text-xs text-[#858b96]">加载中...</div>
                        ) : (
                          <>
                            <Input
                              type="text"
                              value={editTitle}
                              onChange={(e) => setEditTitle(e.target.value)}
                              placeholder="书名"
                              className={inputCls}
                              autoFocus
                            />
                            <Input
                              type="text"
                              value={editAuthor}
                              onChange={(e) => setEditAuthor(e.target.value)}
                              placeholder="作者"
                              className={inputCls}
                            />
                            <Textarea
                              value={editDesc}
                              onChange={(e) => setEditDesc(e.target.value)}
                              placeholder="简介"
                              rows={2}
                              className={inputCls + ' min-h-0 resize-none'}
                            />
                            <div className="flex items-center justify-end gap-2">
                              <TooltipIconButton
                                label="取消"
                                className="text-[#858b96] hover:bg-[#303238]"
                                onClick={() => setEditingBookPath(null)}
                              >
                                <X className="h-3.5 w-3.5" />
                              </TooltipIconButton>
                              <TooltipIconButton
                                label="保存"
                                className="text-[#4a4d54] hover:bg-[#4a4d54]/15"
                                disabled={editSaving}
                                onClick={handleSaveEdit}
                              >
                                <Check className="h-3.5 w-3.5" />
                              </TooltipIconButton>
                            </div>
                          </>
                        )}
                      </div>
                    )
                  }

                  return (
                    <div
                      key={book.path}
                      className={`group relative flex items-start gap-2 rounded px-3 py-2 text-xs hover:bg-[#4a4d54]/15 ${
                        isCurrent ? 'text-[#f0f2f5]' : 'text-[#c5c9d1]'
                      }`}
                    >
                      {isCurrent && (
                        <div className="absolute left-0 top-2 bottom-2 w-[3px] rounded-r bg-[#4a4d54]" />
                      )}
                      <button
                        type="button"
                        className="min-w-0 flex-1 pl-1 text-left"
                        onClick={() => handleSwitch(book.path)}
                      >
                        <div className="truncate text-sm font-semibold">{book.name || '未命名书籍'}</div>
                        <div className="mt-0.5 flex items-center gap-2 text-[11px] text-[#858b96]">
                          {book.author && <span>{book.author}</span>}
                          {book.last_opened_at && <span>{relativeTime(book.last_opened_at)}</span>}
                          {isCurrent && <span className="rounded bg-[#4a4d54]/20 px-1 text-[#c5c9d1]">当前</span>}
                        </div>
                        <div className="mt-0.5 truncate text-[11px] text-[#5a5f6b]">{book.path}</div>
                      </button>
                      <div className="flex shrink-0 items-center gap-0.5 pt-0.5">
                        <TooltipIconButton
                          label="编辑信息"
                          className="text-[#858b96] opacity-0 hover:bg-[#4a4d54]/15 hover:text-[#c5c9d1] group-hover:opacity-100"
                          onClick={() => startEdit(book)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </TooltipIconButton>
                        <TooltipIconButton
                          label="移除记录"
                          className="text-[#858b96] opacity-0 hover:bg-red-500/15 hover:text-red-200 group-hover:opacity-100"
                          onClick={() => handleRemove(book.path)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </TooltipIconButton>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </section>

        </div>
      </ScrollArea>
    </div>
  )
}
