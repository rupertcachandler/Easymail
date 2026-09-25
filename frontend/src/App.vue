<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import * as AppBinding from '../wailsjs/go/app/App'
import { EventsOn as WailsEventsOn, WindowFullscreen as WailsFullscreen, WindowUnfullscreen as WailsUnfullscreen } from '../wailsjs/runtime/runtime'

function isWailsNative(): boolean {
  return !!(window as any).go?.app?.App || !!(window as any).runtime
}

const wails = {
  async call(fn: string, ...args: any[]): Promise<any> {
    if (isWailsNative()) {
      const fnMap: Record<string, (...a: any[]) => Promise<any>> = {
        AddAccount: AppBinding.AddAccount,
        UpdateAccount: AppBinding.UpdateAccount,
        RemoveAccount: AppBinding.RemoveAccount,
        ListAccounts: AppBinding.ListAccounts,
        GetFolders: AppBinding.GetFolders,
        SyncFolders: AppBinding.SyncFolders,
        GetEmails: AppBinding.GetEmails,
        GetEmailsByContact: AppBinding.GetEmailsByContact,
        SyncEmails: AppBinding.SyncEmails,
        GetContacts: AppBinding.GetContacts,
        SyncContacts: AppBinding.SyncContacts,
        SearchContacts: AppBinding.SearchContacts,
        GetCalendarEvents: AppBinding.GetCalendarEvents,
        SyncCalendar: AppBinding.SyncCalendar,
        CreateCalendarEvent: AppBinding.CreateCalendarEvent,
        UpdateCalendarEvent: AppBinding.UpdateCalendarEvent,
        DeleteCalendarEvent: AppBinding.DeleteCalendarEvent,
        GetOOFSettings: AppBinding.GetOOFSettings,
        SetOOFSettings: AppBinding.SetOOFSettings,
        Shutdown: AppBinding.Shutdown,
        SendMail: AppBinding.SendMail,
        FetchAttachment: AppBinding.FetchAttachment,
        DownloadAttachment: AppBinding.DownloadAttachment,
        OpenAttachment: AppBinding.OpenAttachment,
        OpenExternal: AppBinding.OpenExternal,
        SaveHTMLAs: AppBinding.SaveHTMLAs,
        GetMessageSource: AppBinding.GetMessageSource,
        GetActivityLog: AppBinding.GetActivityLog,
        GetVersion: AppBinding.GetVersion,
        CopyEmails: AppBinding.CopyEmails,
        MoveEmails: AppBinding.MoveEmails,
        DeleteEmails: AppBinding.DeleteEmails,
      }
      if (fnMap[fn]) return await fnMap[fn](...args)
    }
    console.warn('[BOI] Binding not available:', fn)
    return null
  },
  on(event: string, handler: (...args: any[]) => void) {
    if (isWailsNative()) WailsEventsOn(event, handler)
  }
}

// === Types ===
interface Account { id: string; name: string; email: string; connected: boolean }
// ACCOUNT_COLOURS: deterministic per-email palette (hash email -> colour) so the
// same mailbox shows the same colour in every BOI copy, no server storage needed.
const ACCOUNT_COLOURS = ['#e6194b','#3b75c3','#f5a623','#66cc00','#ac5ac2','#ff7f0e','#00bcd4','#d62728','#2ca02c','#9467bd','#17becf','#ffbb78']
const accountColour = (email: string): string => {
  const e = (email || '').trim().toLowerCase()
  let h = 0
  for (let i = 0; i < e.length; i++) h = (h * 31 + e.charCodeAt(i)) % 9973
  return ACCOUNT_COLOURS[h % ACCOUNT_COLOURS.length]
}
const accountEmail = (accountId: string): string => accounts.value.find(a => a.id === accountId)?.email || ''
interface Folder { id: string; accountId: string; serverId: string; parentId: string; name: string; type: number; unreadCount: number; isHidden: boolean }
interface EmailAttachment { displayName: string; fileReference: string; contentId: string; isInline: boolean; method: number; estimatedDataSize: number }
interface Email { id: string; accountId: string; folderId: string; serverId: string; from: string; fromEmail: string; to: string; toEmails: string[]; subject: string; dateReceived: any; isRead: boolean; isFlagged: boolean; importance: number; hasAttachment: boolean; attachments?: EmailAttachment[]; preview: string; body: string; bodyType: string }
interface Contact { id: string; accountId: string; name: string; email: string; emailCount: number; isFavorite: boolean }
interface CalendarEvent { id: string; subject: string; startTime: any; endTime: any; location: string; allDayEvent: boolean; busyStatus: number; organizerName: string }

// === State ===
const accounts = ref<Account[]>([])
const folders = ref<Folder[]>([])
const allEmails = ref<Email[]>([])
const contacts = ref<Contact[]>([])
const calendarEvents = ref<CalendarEvent[]>([])
const selectedAccount = ref('')
const selectedFolder = ref('')
const selectedPerson = ref<string|null>(null)
const selectedEmail = ref<Email|null>(null)
const searchQuery = ref('')
const sortBy = ref<'date'|'name'>('date')
const sortDir = ref<'asc'|'desc'>('desc')
const syncing = ref(false)
const error = ref('')

const SYNCABLE_TYPES = [2, 3, 4, 5, 12] // Inbox, Drafts, Trash, Sent, Custom/Junk
const syncedFolderIds = ref<string[]>([])
// Persist the user's folder-sync selection so it survives restarts (mirrors
// signatures). Without this the list is empty again on every boot and the
// mail view shows "No emails yet" until the user re-ticks folders.
function persistSyncedFolders() {
  localStorage.setItem('boi-synced-folders-v2-' + selectedAccount.value, JSON.stringify(syncedFolderIds.value))
}
function restoreSyncedFolders() {
  try {
    const v = JSON.parse(localStorage.getItem('boi-synced-folders-v2-' + selectedAccount.value) || '[]')
    if (Array.isArray(v)) syncedFolderIds.value = v.filter(x => typeof x === 'string')
  } catch { syncedFolderIds.value = [] }
}
const showFolderPanel = ref(false)

// Views: Contacts + Diary (calendar)
const showContacts = ref(false)
const showDiary = ref(false)

type Suggestion = { address: string; label: string }

// Autocomplete source: every EAS contact + every remembered address, deduped
// by (lowercased) address. Remembered addresses land in the same contacts
// table via the Go-side upsert on email save, so this covers both.
const addressSuggestions = computed<Suggestion[]>(() => {
  const seen = new Set<string>()
  const out: Suggestion[] = []
  const push = (addr: string, name: string | undefined) => {
    const a = (addr || '').trim().toLowerCase()
    if (!a || seen.has(a)) return
    seen.add(a)
    out.push({ address: addr.trim(), label: name ? `${name} <${addr.trim()}>` : addr.trim() })
  }
  for (const c of contacts.value) {
    push(c.email, c.name)
    if (c.email2) push(c.email2, c.name)
    if (c.email3) push(c.email3, c.name)
  }
  for (const e of allEmails.value) {
    if (!isFromMe(e)) { push(e.fromEmail || e.from, e.from) }
    for (const t of (e.toEmails || [])) { if (t) push(t, e.to) }
  }
  out.sort((x, y) => x.label.localeCompare(y.label))
  return out.slice(0, 500)
})

// Filtered compose-To suggestions: only show matches for what's currently
// typed (name or address), capped so the dropdown stays snappy. Empty query
// (or a full address) shows none rather than a wall of 500.
const composeSuggestions = computed<Suggestion[]>(() => {
  const q = composeTo.value.trim().toLowerCase().replace(/,.*$/, '')
  if (!q || q.includes('@')) return []
  const hits = addressSuggestions.value
    .filter(s => s.address.toLowerCase().includes(q) || s.label.toLowerCase().includes(q))
    .slice(0, 8)
  return hits
})
const showComposeSug = ref(false)
function pickSuggestion(s: Suggestion) {
  composeTo.value = s.address
  showComposeSug.value = false
}
function onComposeToBlur() { setTimeout(() => { showComposeSug.value = false }, 150) }

// New calendar event
const showNewEvent = ref(false)
// editingEvent is non-null when the modal is editing an existing event; null
// when it is creating a new one.
const editingEvent = ref<any>(null)
const newEventSubject = ref('')
const newEventLocation = ref('')
// New-event datetimes are entered as dd/mm/yy date + HH:MM time in plain text
// inputs — native datetime-local follows the system locale (en-US on the box
// shows mm/dd/yyyy, which is "yuk"). We are not American, so dd/mm/yy it is.
const newEventStartDate = ref('')
const newEventStartTime = ref('')
const newEventEndDate = ref('')
const newEventEndTime = ref('')
const newEventAllDay = ref(false)
const newEventInvitees = ref('')
const newEventSaving = ref(false)
const newEventError = ref('')
const calendarFolder = computed(() => folders.value.find(f => f.type === 8) || null)
function dmy(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getDate())}/${p(d.getMonth() + 1)}/${String(d.getFullYear()).slice(-2)}`
}
function hm(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}`
}
function openNewEvent() {
  const c = diaryCursor.value
  const s = new Date(c.getFullYear(), c.getMonth(), c.getDate(), 10, 0)
  const e = new Date(c.getFullYear(), c.getMonth(), c.getDate(), 11, 0)
  editingEvent.value = null
  newEventStartDate.value = dmy(s); newEventStartTime.value = hm(s)
  newEventEndDate.value = dmy(e); newEventEndTime.value = hm(e)
  newEventSubject.value = ''; newEventLocation.value = ''; newEventAllDay.value = false
  newEventInvitees.value = ''
  newEventError.value = ''
  showNewEvent.value = true
}
function closeNewEvent() {
  showNewEvent.value = false
  editingEvent.value = null
}
function editEvent(ev: any) {
  editingEvent.value = ev
  const s = new Date(ev.startTime), e = new Date(ev.endTime || ev.startTime)
  newEventSubject.value = ev.subject || ''
  newEventLocation.value = ev.location || ''
  newEventStartDate.value = dmy(s); newEventStartTime.value = hm(s)
  newEventEndDate.value = dmy(e); newEventEndTime.value = hm(e)
  newEventAllDay.value = !!ev.allDayEvent
  newEventInvitees.value = (ev.attendees || []).join(', ')
  newEventError.value = ''
  showNewEvent.value = true
}
// Parse "dd/mm/yy" + "HH:MM" into a local Date; returns null on bad input.
function parseDmy(d: string, t: string): Date | null {
  const dm = /^(\d{1,2})\/(\d{1,2})\/(\d{2,4})$/.exec((d || '').trim())
  if (!dm) return null
  const tm = /^(\d{1,2}):(\d{2})$/.exec((t || '').trim())
  if (!tm) return null
  let dd = parseInt(dm[1], 10), mo = parseInt(dm[2], 10), yy = parseInt(dm[3], 10)
  if (yy < 100) yy += yy < 70 ? 2000 : 1900
  const hh = parseInt(tm[1], 10), mm = parseInt(tm[2], 10)
  if (dd < 1 || dd > 31 || mo < 1 || mo > 12 || hh > 23 || mm > 59) return null
  const dt = new Date(yy, mo - 1, dd, hh, mm, 0, 0)
  return (dt.getDate() === dd && dt.getMonth() === mo - 1 && dt.getFullYear() === yy) ? dt : null
}
async function saveNewEvent() {
  if (!newEventSubject.value.trim()) { newEventError.value = 'Subject is required'; return }
  if (!calendarFolder.value) { newEventError.value = 'No calendar folder found for this account'; return }
  const s = parseDmy(newEventStartDate.value, newEventStartTime.value)
  const e = parseDmy(newEventEndDate.value, newEventEndTime.value)
  if (!s || !e) { newEventError.value = 'Enter dates as dd/mm/yy and times as HH:MM (e.g. 14/10/26 09:30)'; return }
  if (e.getTime() <= s.getTime()) { newEventError.value = 'End must be after start'; return }
  newEventSaving.value = true; newEventError.value = ''
  try {
    const ev = {
      subject: newEventSubject.value.trim(),
      location: newEventLocation.value.trim(),
      startTime: s.toISOString(),
      endTime: e.toISOString(),
      allDayEvent: newEventAllDay.value,
      busyStatus: 2, sensitivity: 0,
      attendees: newEventInvitees.value.split(',').map(x => x.trim()).filter(Boolean),
    }
    if (editingEvent.value) {
      await wails.call('UpdateCalendarEvent', selectedAccount.value, calendarFolder.value.serverId,
        editingEvent.value.id, editingEvent.value.serverId, ev)
    } else {
      await wails.call('CreateCalendarEvent', selectedAccount.value, calendarFolder.value.serverId, ev)
    }
    // Refresh so the change shows immediately.
    try { calendarEvents.value = await wails.call('GetCalendarEvents', selectedAccount.value, selectedAccount.value + '-cal') || [] } catch (e) { /* ok */ }
    closeNewEvent()
  } catch (e: any) {
    newEventError.value = e.message || (editingEvent.value ? 'Failed to save changes' : 'Failed to create event')
  } finally {
    newEventSaving.value = false
  }
}
async function deleteEditingEvent() {
  const ev = editingEvent.value
  if (!ev || !ev.serverId) { newEventError.value = 'This event has no server id to delete'; return }
  if (!confirm(`Delete "${ev.subject || '(no subject)'}"? This removes it from your calendar.`)) return
  newEventSaving.value = true; newEventError.value = ''
  try {
    if (calendarFolder.value) {
      await wails.call('DeleteCalendarEvent', selectedAccount.value, calendarFolder.value.serverId, ev.id, ev.serverId)
    }
    try { calendarEvents.value = await wails.call('GetCalendarEvents', selectedAccount.value, selectedAccount.value + '-cal') || [] } catch (e) { /* ok */ }
    closeNewEvent()
  } catch (e: any) {
    newEventError.value = e.message || 'Failed to delete event'
  } finally {
    newEventSaving.value = false
  }
}
function removeEvent(ev: any) {
  editingEvent.value = ev
  deleteEditingEvent()
}

// Compose
const showCompose = ref(false)
const composeTo = ref('')
const composeToEl = ref<HTMLInputElement|null>(null)
const composeCc = ref('')
const composeBcc = ref('')
const showCcBcc = ref(false)
const composeSubject = ref('')
const composeBody = ref('')
const composeSending = ref(false)
const composeError = ref('')
const composeSuccess = ref('')

// Add Account
const showAddAccount = ref(false)
const newAccountName = ref('')
const newAccountEmail = ref('')
const newAccountPassword = ref('')
const DEFAULT_ACTIVESYNC_URL = 'https://example.com/Microsoft-Server-ActiveSync'
const newAccountServer = ref(DEFAULT_ACTIVESYNC_URL)
const addingAccount = ref(false)
const addAccountError = ref('')

// Settings page
const showSettings = ref(false)
const settingsTab = ref<'accounts'|'sync'|'signatures'>('accounts')
function openSettings() { showSettings.value = true }
function openSettingsAccounts() { settingsTab.value = 'accounts'; loadAccounts(); showSettings.value = true }
function closeSettings() { showSettings.value = false }

// Message source viewer: reconstruct the raw source of the currently-open
// email (or, for outbound mail we identity as "You", the source BOI sends)
// and explain each header in plain English.
const showActivity = ref(false)
const activityLines = ref<string[]>([])
const activityLoading = ref(false)
const activityError = ref('')

async function openActivity() {
  showActivity.value = true
  activityError.value = ''
  activityLoading.value = true
  try {
    const lines = await wails.call('GetActivityLog', 300) || []
    activityLines.value = Array.isArray(lines) ? lines : []
  } catch (e: any) {
    activityError.value = e.message || String(e)
  } finally {
    activityLoading.value = false
  }
}
function closeActivity() { showActivity.value = false }

const showSource = ref(false)
const sourceLoading = ref(false)
const sourceError = ref('')
const sourceRaw = ref('')
const sourceFetchedFor = ref('') // id of the email currently fetched

async function openSource() {
  closeSource()
  const e = selectedEmail.value
  if (!e) return
  showSource.value = true
  await loadMessageSource()
}
function closeSource() { showSource.value = false }

// Fetch the REAL raw MIME source from the server, then break it down.
async function loadMessageSource() {
  const e = selectedEmail.value
  if (!e) return
  const key = e.folderId + '|' + e.serverId
  if (sourceFetchedFor.value === key && sourceRaw.value) return // already loaded
  sourceLoading.value = true
  sourceError.value = ''
  try {
    const raw = await wails.call('GetMessageSource', selectedAccount.value, folderServerID(e.folderId), e.serverId)
    sourceRaw.value = String(raw || '')
    sourceFetchedFor.value = key
  } catch (err: any) {
    sourceRaw.value = ''
    sourceFetchedFor.value = ''
    sourceError.value = err?.message || 'Could not fetch message source'
  } finally {
    sourceLoading.value = false
  }
}

// Split raw MIME into folded header lines. Headers never float: the first
// blank line ends the header block; anything after is body.
function parseSourceHeaders(raw: string): { name: string; value: string }[] {
  const rows: { name: string; value: string }[] = []
  const end = raw.indexOf('\n\n')
  const head = (end >= 0 ? raw.slice(0, end) : raw).replace(/\r\n/g, '\n')
  let curName = ''
  let curVal = ''
  const push = () => { if (curName) rows.push({ name: curName, value: curVal }) }
  for (const line of head.split('\n')) {
    if (!line.trim()) continue
    if (/^[ \t]/.test(line)) { curVal += ' ' + line.trim(); continue }
    push()
    const i = line.indexOf(':')
    if (i > 0) { curName = line.slice(0, i).trim(); curVal = line.slice(i + 1).trim() }
  }
  push()
  return rows
}

const parsedSourceHeaders = computed(() => parseSourceHeaders(sourceRaw.value))

// Plain-English translations for the headers. Unknown ones get a generic
// "custom/extension" note so nothing is ever silently skipped.
function hdrName(n: string): string {
  const l = n.toLowerCase()
  const map: Record<string, string> = {
    'return-path': 'Actual return address the system used',
    'received': 'Hops the message took through mail servers',
    'dkim-signature': 'Digital signature confirming sender authenticity',
    'authentication-results': 'Spam/authenticity verdict from the receiving server',
    'subject': 'The topic line',
    'from': 'Sender — who the message is from',
    'to': 'Recipients — who it was addressed to',
    'cc': 'Cc recipients — copied for info',
    'bcc': 'Bcc recipients — hidden copy',
    'date': 'When the message was sent',
    'message-id': 'Unique fingerprint for threading & de-duping',
    'in-reply-to': 'Which earlier message this one answers',
    'references': 'The conversation chain this belongs to',
    'reply-to': 'Address replies should go to',
    'mime-version': 'Email-format standard version',
    'content-type': 'Format of the body (plain text, HTML, multipart…)',
    'content-transfer-encoding': 'How the body was encoded for transit',
    'content-disposition': 'How an attachment should be shown/handled',
    'content-id': 'ID used to link inline images/parts',
    'importance': 'Priority flag set by the sender',
    'priority': 'Priority flag set by the sender',
    'x-priority': 'Priority flag (Outlook-style)',
    'x-mailer': 'The client that sent it — BOI',
    'x-powered-by': "BOI's stamp — from Rupert & Frau Blücher",
    'x-frau-blucher': 'A farewell from Frau Blücher — the horses know what that means. 🐎',
  }
  return map[l] || (l.startsWith('x-') ? 'A custom/extension header — specific to the sender' : 'A standard delivery header')
}

// ---- (fallback) Rebuild a source block from local fields if the server fetch fails ----
function currentSourceText(): string {
  if (sourceRaw.value) return sourceRaw.value
  const e = selectedEmail.value
  if (!e) return ''
  const w = e.id.includes('-') ? e.id.split('-')[1] : e.serverId
  const lines: string[] = []
  lines.push('Return-Path: <' + (e.fromEmail || '') + '>')
  lines.push('From: ' + (e.from || e.fromEmail || ''))
  lines.push('To: ' + (e.to || (e.toEmails || []).join(', ')))
  if (e.cc) lines.push('Cc: ' + e.cc)
  lines.push('Subject: ' + (e.subject || ''))
  lines.push('Date: ' + formatRFC(e.dateReceived))
  lines.push('Message-ID: <' + w + '@boi>')
  lines.push('MIME-Version: 1.0')
  lines.push('Content-Type: text/' + (e.bodyType === 'html' ? 'html' : 'plain') + '; charset=utf-8')
  return lines.join('\n')
}

function formatRFC(d: string): string {
  const t = new Date(d)
  if (isNaN(t.getTime())) return d
  return t.toUTCString().replace('GMT', '+0000 (UTC)')
}

const explainableHeaders = [
  { name: 'From',               desc: 'Who the message is from — the sender&apos;s name/address.' },
  { name: 'To / Cc',            desc: 'Who it was sent to (Cc = copy, for info).' },
  { name: 'Subject',            desc: 'The topic line.' },
  { name: 'Date',               desc: 'When it was sent.' },
  { name: 'Message-ID',         desc: 'A unique fingerprint so mail servers can thread and de-duplicate it.' },
  { name: 'MIME-Version',       desc: 'Tells readers which email-format standard to use.' },
  { name: 'Content-Type',       desc: 'Says whether the body is plain text or HTML.' },
  { name: 'X-Mailer',           desc: 'The client that sent it — here, BOI.' },
  { name: 'X-Powered-By',       desc: 'BOI&apos;s stamp — sent by Rupert &amp; their AI assistant, Frau Blücher.' },
  { name: 'X-Frau-Blucher',     desc: 'A farewell from Frau Blücher — the horses know what that means. 🐎' },
]

// Account editor (reuses the same fields, but with an edit target)
const editingAccountId = ref('')
const editingAccount = ref<{id:string,name:string,email:string,serverUrl:string}|null>(null)
function startAccountEditor(a: any) {
  if (!a) { startNewAccount() ; return }
  editingAccountId.value = a.id
  newAccountName.value = a.name || ''
  newAccountEmail.value = a.email || ''
  newAccountPassword.value = ''  // never prefill the password
  newAccountServer.value = a.serverUrl || DEFAULT_ACTIVESYNC_URL
  addAccountError.value = ''
}
function startNewAccount() {
  editingAccountId.value = ''
  newAccountName.value = ''; newAccountEmail.value = ''; newAccountPassword.value = ''
  newAccountServer.value = DEFAULT_ACTIVESYNC_URL
  addAccountError.value = ''
}
async function saveAccount() {
  addingAccount.value = true; addAccountError.value = ''
  try {
    if (editingAccountId.value) {
      await wails.call('UpdateAccount', editingAccountId.value, newAccountName.value, newAccountEmail.value, newAccountPassword.value, newAccountServer.value)
    } else {
      await wails.call('AddAccount', newAccountName.value, newAccountEmail.value, newAccountPassword.value, newAccountServer.value)
    }
    accounts.value = await wails.call('ListAccounts') || []
    if (editingAccountId.value) {
      // keep selection; reconnect covers the rest
    } else if (accounts.value.length) {
      selectedAccount.value = accounts.value[0].id
    }
    startNewAccount()
    await syncAll()
  } catch (e: any) { addAccountError.value = e.message || String(e) }
  addingAccount.value = false
}
async function deleteAccount(a: any) {
  if (!confirm(`Remove account \"${a.email}\"? This deletes its cached mail.`)) return
  try {
    await wails.call('RemoveAccount', a.id)
    accounts.value = await wails.call('ListAccounts') || []
    if (selectedAccount.value === a.id) selectedAccount.value = accounts.value[0]?.id || ''
    if (accounts.value.length) await syncAll()
  } catch (e: any) { addAccountError.value = e.message || String(e) }
}

// === UNIBOX: People-centric grouping (sent + received merged) ===
interface PersonInfo { email: string; name: string; lastDate: Date; count: number; hasUnread: boolean }

const myEmail = computed(() => {
  const acc = accounts.value.find(a => a.id === selectedAccount.value)
  return acc?.email?.toLowerCase() || ''
})

function isFromMe(e: Email): boolean {
  const from = (e.fromEmail || e.from).toLowerCase()
  return from === myEmail.value || from.includes(myEmail.value.split('@')[0])
}

function otherPerson(e: Email): string {
  if (isFromMe(e)) {
    return (e.toEmails && e.toEmails.length > 0) ? e.toEmails[0].toLowerCase() : e.to.toLowerCase()
  }
  return (e.fromEmail || e.from).toLowerCase()
}

function otherPersonName(e: Email): string {
  if (isFromMe(e)) {
    return e.to ? e.to.split(',')[0].replace(/<.*>/, '').trim() : e.toEmails?.[0] || ''
  }
  return e.from || e.fromEmail
}

const peopleGroups = computed(() => {
  const pool = selectedFolder.value
    ? allEmails.value.filter(e => e.folderId === selectedFolder.value)
    : allEmails.value

  const q = searchQuery.value.toLowerCase()
  const emails = q
    ? pool.filter(e =>
        e.from.toLowerCase().includes(q) ||
        (e.fromEmail || '').toLowerCase().includes(q) ||
        e.subject.toLowerCase().includes(q) ||
        (e.to || '').toLowerCase().includes(q) ||
        (e.toEmails || []).some(t => t.toLowerCase().includes(q))
      )
    : pool

  const byPerson = new Map<string, PersonInfo>()
  for (const e of emails) {
    const key = otherPerson(e)
    const d = new Date(e.dateReceived)
    const name = otherPersonName(e)
    const existing = byPerson.get(key)
    if (!existing) {
      byPerson.set(key, { email: key, name, lastDate: d, count: 1, hasUnread: !e.isRead })
    } else {
      existing.count++
      if (d > existing.lastDate) existing.lastDate = d
      if (!e.isRead) existing.hasUnread = true
    }
  }
  return Array.from(byPerson.values()).sort((a, b) => b.lastDate.getTime() - a.lastDate.getTime())
})

// Sort the people list by date or by name, honouring the direction chosen.
const sortedPeople = computed(() => {
  const list = [...peopleGroups.value]
  if (sortBy.value === 'name') {
    const byName = (a: PersonInfo, b: PersonInfo) => (a.name || a.email || '').localeCompare(b.name || b.email || '')
    list.sort((a, b) => sortDir.value === 'asc' ? byName(a, b) : byName(b, a))
  } else {
    list.sort((a, b) => sortDir.value === 'desc'
      ? b.lastDate.getTime() - a.lastDate.getTime()
      : a.lastDate.getTime() - b.lastDate.getTime())
  }
  return list
})

const sortLabel = computed(() =>
  sortBy.value === 'name'
    ? (sortDir.value === 'asc' ? 'A→Z' : 'Z→A')
    : (sortDir.value === 'desc' ? 'New→Old' : 'Old→New')
)

const todayStart = computed(() => { const d = new Date(); d.setHours(0,0,0,0); return d })
const yesterdayStart = computed(() => new Date(todayStart.value.getTime() - 86400000))

// Group the (already sorted) list by recency. When direction is old→new the
// header order reads Bottom-up but items within each group stay in the chosen
// order; that is acceptable and matches the toggle.
const todayPeople = computed(() => sortedPeople.value.filter(s => s.lastDate >= todayStart.value))
const yesterdayPeople = computed(() => sortedPeople.value.filter(s => s.lastDate >= yesterdayStart.value && s.lastDate < todayStart.value))
const earlierPeople = computed(() => sortedPeople.value.filter(s => s.lastDate < yesterdayStart.value))

// Grouped list for the date view: [Today, Yesterday, Earlier]. Rendering one
// loop for all three removes the copy-pasted template blocks and keeps the
// empty-group handling in one place.
const senderGroups = computed(() => {
  const groups: { label: string; people: PersonInfo[] }[] = []
  if (todayPeople.value.length) groups.push({ label: 'Today', people: todayPeople.value })
  if (yesterdayPeople.value.length) groups.push({ label: 'Yesterday', people: yesterdayPeople.value })
  if (earlierPeople.value.length) groups.push({ label: 'Earlier', people: earlierPeople.value })
  return groups
})

// All emails exchanged with the selected person (both sent and received)
const personEmails = computed(() => {
  if (!selectedPerson.value) return []
  const pool = selectedFolder.value
    ? allEmails.value.filter(e => e.folderId === selectedFolder.value)
    : allEmails.value
  return pool
    .filter(e => otherPerson(e) === selectedPerson.value)
    .sort((a, b) => new Date(b.dateReceived).getTime() - new Date(a.dateReceived).getTime())
})

const mailFolders = computed(() => folders.value.filter(f => !f.isHidden && SYNCABLE_TYPES.includes(f.type)))

// === Folder tree ===
// parentId drives a nested tree so subfolders render indented under their
// parent (the data was always in the DB, this is purely display).
// Folder hierarchy is keyed by SERVER id: the DB stores folders.parent_id as
// the parent's server_id (e.g. mail%2F...), NOT the local row id. So the tree
// must match f.parentId against the parent's serverId; '', '0' or a dangling
// parent id all count as a root (folder present but parent not in the list).
const folderChildren = (parentServerId: string): Folder[] => {
  return mailFolders.value.filter(f => (f.parentId || '') === parentServerId)
}
const folderRoots = computed(() => mailFolders.value.filter((f) => {
  const p = f.parentId || ''
  return p === '' || p === '0' || !mailFolders.value.some(x => x.serverId === p)
}))
// Flat indented list used by the flat pickers (folder manager / move target).
const indentedFolders = computed(() => {
  const out: { f: Folder; depth: number }[] = []
  const walk = (parentServerId: string, depth: number) => {
    for (const f of folderChildren(parentServerId)) {
      out.push({ f, depth })
      walk(f.serverId, depth + 1)
    }
  }
  for (const f of folderRoots.value) { out.push({ f, depth: 0 }); walk(f.serverId, 1) }
  return out
})
const folderIcon = (f: Folder): string => f.type === 2 ? '📥' : f.type === 5 ? '📤' : f.type === 3 ? '📝' : f.type === 4 ? '🗑' : '📁'
const folderCollapsed = ref<Set<string>>(new Set())
const folderHasChildren = (f: Folder): boolean => folderChildren(f.serverId).length > 0
const toggleFolder = (f: Folder) => {
  const s = new Set(folderCollapsed.value)
  if (s.has(f.id)) s.delete(f.id); else s.add(f.id)
  folderCollapsed.value = s
}
// Flattened tree for the sync list, honouring collapsed state at any depth.
const visibleFolderTree = computed(() => {
  const out: { f: Folder; depth: number }[] = []
  const walk = (parentServerId: string, depth: number) => {
    for (const f of folderChildren(parentServerId)) {
      out.push({ f, depth })
      if (!folderCollapsed.value.has(f.id)) walk(f.serverId, depth + 1)
    }
  }
  for (const f of folderRoots.value) {
    out.push({ f, depth: 0 })
    if (!folderCollapsed.value.has(f.id)) walk(f.serverId, 1)
  }
  return out
})

// === Avatar ===
const avatarUrl = (email: string, size = 64) => {
  if (!email) return `https://www.gravatar.com/avatar/?d=mp&s=${size}`
  // Gravatar/Libravatar key avatars by MD5 hex of the trim+lowercased email.
  // The old FNV-1a hash was wrong, so everyone fell back to identicon.
  const hash = md5Hex(email.trim().toLowerCase())
  return `https://seccdn.libravatar.org/avatar/${hash}?s=${size}&d=identicon`
}

function md5Hex(s: string): string {
  const k = [0xd76aa478,0xe8c7b756,0x242070db,0xc1bdceee,0xf57c0faf,0x4787c62a,0xa8304613,0xfd469501,0x698098d8,0x8b44f7af,0xffff5bb1,0x895cd7be,0x6b901122,0xfd987193,0xa679438e,0x49b40821,0xf61e2562,0xc040b340,0x265e5a51,0xe9b6c7aa,0xd62f105d,0x02441453,0xd8a1e681,0xe7d3fbc8,0x21e1cde6,0xc33707d6,0xf4d50d87,0x455a14ed,0xa9e3e905,0xfcefa3f8,0x676f02d9,0x8d2a4c8a,0xfffa3942,0x8771f681,0x6d9d6122,0xfde5380c,0xa4beea44,0x4bdecfa9,0xf6bb4b60,0xbebfbc70,0x289b7ec6,0xeaa127fa,0xd4ef3085,0x04881d05,0xd9d4d039,0xe6db99e5,0x1fa27cf8,0xc4ac5665,0xf4292244,0x432aff97,0xab9423a7,0xfc93a039,0x655b59c3,0x8f0ccc92,0xffeff47d,0x85845dd1,0x6fa87e4f,0xfe2ce6e0,0xa3014314,0x4e0811a1,0xf7537e82,0xbd3af235,0x2ad7d2bb,0xeb86d391]
  const r = [7,12,17,22,7,12,17,22,7,12,17,22,7,12,17,22,5,9,14,20,5,9,14,20,5,9,14,20,5,9,14,20,4,11,16,23,4,11,16,23,4,11,16,23,4,11,16,23,6,10,15,21,6,10,15,21,6,10,15,21,6,10,15,21]
  const bytes = new TextEncoder().encode(s)
  const bitLen = bytes.length * 8
  // Correct MD5 padding: append 0x80, zeros, then the 64-bit length, so the
  // total is a multiple of 64 bytes. (The earlier version allocated a buffer
  // too small and threw RangeError on padded.set() for any email >14 chars,
  // crashing the whole render — that was the white screen.)
  const el = ((bitLen >>> 3) + 1 + 8 + 63) & ~63
  const padded = new Uint8Array(el)
  padded.set(bytes)
  padded[bytes.length] = 0x80
  const dv = new DataView(padded.buffer)
  dv.setUint32(el - 8, bitLen >>> 0, true)
  dv.setUint32(el - 4, Math.floor(bitLen / 0x100000000), true)
  let a0 = 0x67452301, b0 = 0xefcdab89, c0 = 0x98badcfe, d0 = 0x10325476
  for (let i = 0; i < el; i += 64) {
    const w: number[] = []
    for (let j = 0; j < 16; j++) w[j] = dv.getUint32(i + j * 4, true)
    let A = a0, B = b0, C = c0, D = d0
    for (let j = 0; j < 64; j++) {
      let F, g
      if (j < 16) { F = (B & C) | (~B & D); g = j }
      else if (j < 32) { F = (D & B) | (~D & C); g = (5 * j + 1) % 16 }
      else if (j < 48) { F = B ^ C ^ D; g = (3 * j + 5) % 16 }
      else { F = C ^ (B | ~D); g = (7 * j) % 16 }
      F = (F + A + k[j] + w[g]) | 0
      A = D; D = C; C = B
      B = (B + ((F << r[j]) | (F >>> (32 - r[j])))) | 0
    }
    a0 = (a0 + A) | 0; b0 = (b0 + B) | 0; c0 = (c0 + C) | 0; d0 = (d0 + D) | 0
  }
  // MD5 emits each 32-bit word least-significant-byte first.
  const hexLE = (n: number) => {
    const b = n >>> 0
    const h = ('00000000' + (n >>> 0).toString(16)).slice(-8)
    return h.slice(6, 8) + h.slice(4, 6) + h.slice(2, 4) + h.slice(0, 2)
  }
  return hexLE(a0) + hexLE(b0) + hexLE(c0) + hexLE(d0)
}

// === Formatting ===
const formatTime = (d: string) => { if (!d) return ''; const dt = new Date(d); return dt.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' }) }
// Always dd/mm/yy — we are not American.
const ddmmYY = (dt: Date): string => {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(dt.getDate())}/${p(dt.getMonth() + 1)}/${String(dt.getFullYear()).slice(-2)}`
}
const formatDate = (d: string) => { if (!d) return ''; const dt = new Date(d); if (isNaN(dt.getTime())) return ''; return ddmmYY(dt) }
const formatShort = (d: string) => {
  if (!d) return ''
  const dt = new Date(d); const now = new Date(); const days = Math.floor((now.getTime() - dt.getTime()) / 86400000)
  if (days === 0) return formatTime(d)
  if (days === 1) return 'Yesterday'
  if (days < 7) return dt.toLocaleDateString('en-GB', { weekday: 'short' })
  return ddmmYY(dt)
}
const strippreview = (s: string) => (s || '').replace(/<[^>]+>/g, '')

// === Actions ===
async function loadAccounts() {
  try {
    accounts.value = await wails.call('ListAccounts') || []
    if (accounts.value.length > 0 && !selectedAccount.value) {
      selectedAccount.value = accounts.value[0].id
      await syncAll()
    }
  } catch (e: any) { error.value = e.message }
}

// Guarded entry: never run two drains at once (SOGo returns 405 and the
// loser aborts mid-window, leaving old mail on screen).
async function syncAll() {
  if (!selectedAccount.value) return
  if (syncing.value) { syncQueued = true; return }
  syncing.value = true; error.value = ''
  try { await syncAllLocked() } finally {
    syncing.value = false
    if (syncQueued) { syncQueued = false; syncAll() }
  }
}

// loadAllEmails pulls the whole of every synced folder from the local cache
// (instant — no network). It's called before syncing so previous mail shows
// as soon as the app opens, then again once all folders have finished
// syncing so the final result settles cleanly. The local array is built in
// full and assigned once at the end — never cleared mid-build — which stops
// the list flashing blank between folders while syncing.
async function loadAllEmails() {
  const mf = folders.value.filter(f => SYNCABLE_TYPES.includes(f.type))
  const toSync = mf.filter(f => syncedFolderIds.value.includes(f.id))
  const all: Email[] = []
  for (const f of toSync) {
    try {
      // Load the WHOLE folder, not a 200-item slice — the old cap hid
      // everything beyond the newest 200 (e.g. 1,899 INBOX emails ⇒ ~90%
      // never shown, and today's mail past the slice invisible).
      const em = await wails.call('GetEmails', selectedAccount.value, f.id, 0, 10000) || []
      all.push(...em)
    } catch (e) { console.error('[BOI] load emails failed for', f.name, e) }
  }
  allEmails.value = all
}
// Reload a single folder's mail from cache and merge it into the full list —
// used by the emails-updated event so a background poll round only touches
// the folder that changed, instead of re-fetching every folder and re-
// rendering the whole list (the old loadAllEmails() on every poll round froze
// the UI for seconds on this box).
async function reloadFolderEmails(folder: any) {
  try {
    const em = await wails.call('GetEmails', selectedAccount.value, folder.id, 0, 10000) || []
    const other = allEmails.value.filter(e => e.folderId !== folder.id)
    // Keep the full list sorted newest-first, matching the main view.
    allEmails.value = other.concat(em).sort((a, b) => new Date(b.dateReceived).getTime() - new Date(a.dateReceived).getTime())
  } catch (e) { console.error('[BOI] reload emails failed for', folder.name, e) }
}
async function syncAllLocked() {
  try {
    // Startup race: Go connects accounts in a background goroutine. The very
    // first SyncFolders can hit 'account not connected' and abort the whole
    // sync (→ empty mailbox). Retry briefly — the connect goroutine usually
    // finishes within a couple of seconds.
    let syncErr: any = null
    for (let attempt = 1; attempt <= 5; attempt++) {
      try {
        await wails.call('SyncFolders', selectedAccount.value)
        syncErr = null
        break
      } catch (e) {
        syncErr = e
        if (attempt < 5) await new Promise(r => setTimeout(r, 1200 * attempt))
      }
    }
    if (syncErr) throw syncErr
    folders.value = await wails.call('GetFolders', selectedAccount.value) || []

    const mf = folders.value.filter(f => SYNCABLE_TYPES.includes(f.type))
    // Default to Inbox + Sent for everyday conversations; the user can tick
    // more in Settings → Mail & Folders (persisted in localStorage).
    if (syncedFolderIds.value.length === 0) {
      const inbox = mf.find(f => f.type === 2)
      const sent = mf.find(f => f.type === 5)
      if (inbox) syncedFolderIds.value.push(inbox.id)
      if (sent) syncedFolderIds.value.push(sent.id)
      persistSyncedFolders()
    }
    // Show yesterday's mail immediately from the local cache, then sync every
    // folder in the background, then refresh once at the end so the list
    // settles on the final result. Refreshing only once (instead of after
    // every folder) stops the flicker while syncing.
    await loadAllEmails()

    const toSync = mf.filter(f => syncedFolderIds.value.includes(f.id))
    for (const f of toSync) {
      try { await wails.call('SyncEmails', selectedAccount.value, f.serverId) } catch (e2) { console.error('[BOI] sync emails failed for', f.name, e2) }
    }
    await loadAllEmails()

    // Contacts (type 9) + Calendar (type 8): sync each, then load.
    const cc = folders.value.find(f => f.type === 9)
    if (cc) { try { await wails.call('SyncContacts', selectedAccount.value, cc.serverId) } catch (e) { /* ok */ } }
    try { contacts.value = await wails.call('SearchContacts', selectedAccount.value, '') || [] } catch (e) { contacts.value = [] }

    const cal = folders.value.find(f => f.type === 8)
    if (cal) { try { await wails.call('SyncCalendar', selectedAccount.value, cal.serverId) } catch (e) { /* ok */ } }
    try { calendarEvents.value = await wails.call('GetCalendarEvents', selectedAccount.value, selectedAccount.value + '-cal') || [] } catch (e) { calendarEvents.value = [] }

    try { const oof = await wails.call('GetOOFSettings', selectedAccount.value); if (oof) oofSettings.value = oof } catch (e) { /* ok */ }
  } catch (e: any) { error.value = e.message }
}

function selectPerson(email: string) { selectedPerson.value = email; selectedEmail.value = null; selectedEmailIds.value = []; moveMenuOpen.value = false }
function selectEmail(email: Email) { selectedEmail.value = email }
function backToPeople() { selectedPerson.value = null; selectedEmail.value = null; selectedEmailIds.value = [] }
function backToConversation() { selectedEmail.value = null; moveMenuOpen.value = false }

// ---- Multi-select + mail management (delete / move / create folder) ----
const selectedEmailIds = ref<string[]>([])
const folderMoveTarget = ref('')
const newFolderName = ref('')
const newFolderParentId = ref('') // '' = mailbox root; else the parent mail folder id
const folderBusy = ref(false)
const moveError = ref('')

// moveTargets is what Settings → Mail & Folders operates on: the emails the
// user highlighted on the mail page (Ctrl/Cmd-click). Nothing is carried
// between pages — Settings simply reflects the live selection.
const moveTargets = computed<Email[]>(() => {
  const sel = new Set(selectedEmailIds.value)
  return personEmails.value.filter(e => sel.has(e.id))
})

function folderServerID(fid: string): string {
  return folders.value.find(f => f.id === fid)?.serverId || ''
}

// ====== Settings "Mail & Folders" → per-folder mail manager ======
// Lets you pick ANY mail folder, see what's in it, Select single or many, and
// Copy / Move / Delete — all from within Settings, no need to pre-select on
// the mail page first.
const managerShow = ref(false)
const managerFolderId = ref('')
const managerEmails = ref<Email[]>([])
const managerLoading = ref(false)
const managerSel = ref<string[]>([]) // selected email ids (folder-qualified)
const managerDstFolderId = ref('')
const managerBusy = ref(false)
const managerError = ref('')
const managerMsg = ref('')

async function managerOpen() {
  managerShow.value = true
  managerError.value = ''; managerMsg.value = ''; managerSel.value = []
  // Default to the first syncable folder (Inbox when present).
  if (!managerFolderId.value && mailFolders.value.length) {
    const inbox = mailFolders.value.find(f => f.type === 2) || mailFolders.value[0]
    managerFolderId.value = inbox.id
  }
  await managerLoad()
}
function managerClose() { managerShow.value = false; managerSel.value = [] }

async function managerLoad() {
  if (!managerFolderId.value) { managerEmails.value = []; return }
  managerLoading.value = true; managerError.value = ''; managerSel.value = []
  try {
    const em = await wails.call('GetEmails', selectedAccount.value, managerFolderId.value, 0, 500)
    managerEmails.value = em || []
  } catch (err: any) {
    managerError.value = err?.message || 'Could not load folder'
    managerEmails.value = []
  } finally { managerLoading.value = false }
}

function managerToggle(id: string) {
  const i = managerSel.value.indexOf(id)
  if (i >= 0) managerSel.value.splice(i, 1)
  else managerSel.value.push(id)
}
function managerSelectAll() {
  managerSel.value = managerEmails.value.length && managerSel.value.length === managerEmails.value.length
    ? [] : managerEmails.value.map(e => e.id + '\u0000' + e.serverId)
}
const managerAllChecked = computed(() =>
  managerEmails.value.length > 0 && managerSel.value.length === managerEmails.value.length)

// Any selected emails in the current manager folder.
function managerItems(): { id: string; email: Email }[] {
  const em = managerEmails.value
  const keyMap = new Set(managerSel.value)
  return em.filter(e => keyMap.has(e.id + '\u0000' + e.serverId)).map(e => ({ id: e.id, email: e }))
}

async function managerDelete() {
  const items = managerItems()
  if (!items.length) { managerError.value = 'Select at least one email (or tick Select all).'; return }
  if (!confirm(`Delete ${items.length} email${items.length > 1 ? 's' : ''}? This moves them to Trash.`)) return
  managerBusy.value = true; managerError.value = ''; managerMsg.value = ''
  try {
    await wails.call('DeleteEmails', selectedAccount.value, items.map(it => ({ serverId: it.email.serverId, folderId: folderServerID(it.email.folderId) })))
    managerMsg.value = `Deleted ${items.length} to Trash.`
    await managerLoad()
  } catch (err: any) { managerError.value = err?.message || 'Delete failed' }
  finally { managerBusy.value = false }
}

async function managerCopyOrMove(op: 'copy' | 'move') {
  const items = managerItems()
  const dst = folderServerID(managerDstFolderId.value)
  if (!items.length) { managerError.value = 'Select at least one email (or tick Select all).'; return }
  if (!managerDstFolderId.value || !dst) { managerError.value = 'Choose a destination folder first.'; return }
  managerBusy.value = true; managerError.value = ''; managerMsg.value = ''
  try {
    const payload = items.map(it => ({ serverId: it.email.serverId, srcFolderId: folderServerID(it.email.folderId), dstFolderId: dst }))
    await wails.call(op === 'copy' ? 'CopyEmails' : 'MoveEmails', selectedAccount.value, payload)
    managerMsg.value = `${op === 'copy' ? 'Copied' : 'Moved'} ${items.length} email${items.length > 1 ? 's' : ''}.`
    managerSel.value = []
    if (op === 'move') await managerLoad() // moved items leave this folder
  } catch (err: any) {
    managerError.value = err?.message || (op === 'copy' ? 'Copy failed' : 'Move failed')
  } finally { managerBusy.value = false }
}

function fmtMailmanSubject(e: Email): string { return e.subject || '(no subject)' }

// Click a bubble: plain click opens it; ctrl/cmd toggles multi-select without
// opening; shift extends to a single email. Selection surfaces on the mail
// page as blue rings; manipulation happens under Settings → Mail & Folders.
function onBubbleClick(e: Email, ev: MouseEvent) {
  if (ev.ctrlKey || ev.metaKey) {
    toggleSelect(e)
    return
  }
  if (ev.shiftKey) {
    selectEmail(e)
    return
  }
  selectedEmailIds.value = []
  selectEmail(e)
}
function toggleSelect(e: Email) {
  const i = selectedEmailIds.value.indexOf(e.id)
  if (i >= 0) selectedEmailIds.value.splice(i, 1)
  else selectedEmailIds.value.push(e.id)
}

function selectedEmails(): Email[] {
  const sel = new Set(selectedEmailIds.value)
  return personEmails.value.filter(e => sel.has(e.id))
}

// Delete: Settings → Mail & Folders → Delete (single or the highlighted group).
async function deleteEmails() {
  const items = selectedEmails()
  if (items.length === 0) { moveError.value = 'Select emails on the mail page first (Ctrl/Cmd-click).'; return }
  const confirmed = confirm(`Delete ${items.length} email${items.length > 1 ? 's' : ''}? This moves them to Trash.`)
  if (!confirmed) return
  try {
    await wails.call('DeleteEmails', selectedAccount.value, items.map(e => ({ serverId: e.serverId, folderId: folderServerID(e.folderId) })))
    selectedEmailIds.value = []
    folderMoveTarget.value = ''
    await loadAllEmails()
  } catch (err: any) { moveError.value = err?.message || 'Delete failed' }
}

// Move: executed from Settings "Mail & Folders" for the emails selected on the
// mail page.
async function moveToFolder(fid: string) {
  const items = selectedEmails()
  const dst = folderServerID(fid)
  if (!dst || items.length === 0) { moveError.value = 'Select emails on the mail page first (Ctrl/Cmd-click).'; return }
  folderBusy.value = true; moveError.value = ''
  try {
    await wails.call('MoveEmails', selectedAccount.value, items.map(e => ({ serverId: e.serverId, srcFolderId: folderServerID(e.folderId), dstFolderId: dst })))
    selectedEmailIds.value = []
    folderMoveTarget.value = ''
    await loadAllEmails()
  } catch (err: any) { moveError.value = err?.message || 'Move failed' }
  finally { folderBusy.value = false }
}

// --- Inline single-email actions (reading pane) ---

// Folders an open email may be moved/copied to: all mail folders except the
// account's Trash (moving mail into Trash is what Delete does).
const moveDestFolders = computed(() => mailFolders.value.filter(f => f.type !== 4))
const moveMenuOpen = ref(false)

// Move the currently-open email into a destination folder (folder is the EAS
// destination server id).
async function moveSingleEmail(dstServerId: string) {
  moveMenuOpen.value = false
  const e = selectedEmail.value
  if (!e || !dstServerId) return
  const srcFolderId = folderServerID(e.folderId)
  moveError.value = ''
  try {
    await wails.call('MoveEmails', selectedAccount.value,
      [{ serverId: e.serverId, srcFolderId, dstFolderId: dstServerId }])
    await loadAllEmails()
    backToConversation()
  } catch (err: any) { moveError.value = err?.message || 'Move failed' }
}

// Delete the currently-open email (moves it to Trash).
async function deleteSingleEmail() {
  const e = selectedEmail.value
  if (!e) return
  if (!confirm('Delete this email? It moves to Trash.')) return
  moveError.value = ''
  try {
    await wails.call('DeleteEmails', selectedAccount.value,
      [{ serverId: e.serverId, folderId: folderServerID(e.folderId) }])
    await loadAllEmails()
    backToConversation()
  } catch (err: any) { moveError.value = err?.message || 'Delete failed' }
}

// --- Inline actions from the conversation list ---

// Per-bubble quick actions: move/delete ONE email straight from the list
// without opening it first.
async function moveListEmail(e: Email, dst: HTMLSelectElement) {
  const dstServerId = dst.value
  if (!dstServerId) return
  dst.value = ''
  moveError.value = ''
  try {
    await wails.call('MoveEmails', selectedAccount.value,
      [{ serverId: e.serverId, srcFolderId: folderServerID(e.folderId), dstFolderId: dstServerId }])
    await loadAllEmails()
  } catch (err: any) { moveError.value = err?.message || 'Move failed' }
}

async function deleteListEmail(e: Email) {
  if (!confirm('Delete this email? It moves to Trash.')) return
  moveError.value = ''
  try {
    await wails.call('DeleteEmails', selectedAccount.value,
      [{ serverId: e.serverId, folderId: folderServerID(e.folderId) }])
    await loadAllEmails()
  } catch (err: any) { moveError.value = err?.message || 'Delete failed' }
}

// Group actions: delete or move every currently-selected email at once.
async function deleteSelectedGroup() {
  await deleteEmails()
}

async function moveSelectedGroup(fid: string) {
  await moveToFolder(fid)
}

// Create a new folder, then (optionally) move the picked group into it.
// newFolderParentId is the folder id ('' = mailbox root) the new folder nests
// under, so users can create subfolders.
async function createFolder() {
  const name = newFolderName.value.trim()
  if (!name) { moveError.value = 'Enter a folder name'; return }
  folderBusy.value = true; moveError.value = ''
  try {
    const parentServerId = newFolderParentId.value ? folderServerID(newFolderParentId.value) : '0'
    const serverId = await wails.call('CreateMailFolder', selectedAccount.value, name, parentServerId || '0')
    folders.value = await wails.call('GetFolders', selectedAccount.value) || []
    // moveTargets may be empty if created from an empty state; if there are
    // items, move them straight in.
    if (moveTargets.value.length) {
      const fid = folders.value.find(f => f.serverId === serverId)?.id
      if (fid) await moveToFolder(fid)
    }
    newFolderName.value = ''
    newFolderParentId.value = ''
  } catch (err: any) { moveError.value = err?.message || 'Create folder failed' }
  finally { folderBusy.value = false }
}

function changeFolder(fid: string) { selectedFolder.value = fid; selectedPerson.value = null; selectedEmail.value = null }
function clearFolder() { selectedFolder.value = ''; selectedPerson.value = null; selectedEmail.value = null }
function toggleFolderSync(fid: string) {
  const idx = syncedFolderIds.value.indexOf(fid)
  if (idx >= 0) syncedFolderIds.value.splice(idx, 1)
  else syncedFolderIds.value.push(fid)
  persistSyncedFolders()
  loadAllEmails()
}

// ---- Signatures (named store + per-account default) ----
interface SavedSignature { id: string; name: string; text: string }
const signatures = ref<SavedSignature[]>([])
const composeSigId = ref('')

const sigEditId = ref('')
const sigEditName = ref('')
const sigEditText = ref('')
const sigEditIsNew = ref(false)

function loadSignatures() {
  try { signatures.value = JSON.parse(localStorage.getItem('boi-signatures') || '[]') || [] }
  catch { signatures.value = [] }
}
function persistSignatures() {
  localStorage.setItem('boi-signatures', JSON.stringify(signatures.value))
}

function sigDefaultId(): string {
  return localStorage.getItem('boi-sig-default-' + selectedAccount.value) || ''
}
function setSigDefaultId(id: string) {
  localStorage.setItem('boi-sig-default-' + selectedAccount.value, id)
}
function sigById(id: string): SavedSignature | undefined {
  return signatures.value.find(s => s.id === id)
}

function openCompose(to = '', subject = '', body = '', cc = '') {
  composeTo.value = to; composeSubject.value = subject; composeBody.value = body; composeCc.value = cc; composeBcc.value = ''
  // Preselect this account's default signature ('' = none)
  composeSigId.value = sigDefaultId()
  composeError.value = ''; composeSuccess.value = ''; showCompose.value = true
  // Focus the To field so the composer doesn't sit unfocused (previously the
  // first stray click landed on the overlay's @click.self and closed the
  // modal — the "reply disappears before I can type" bug).
  nextTick(() => { composeToEl.value?.focus() })
}

function signaturePreview() {
  const s = (sigById(composeSigId.value)?.text || '').trim()
  if (!s) return composeBody.value
  return composeBody.value + (composeBody.value.trim() ? '\n' : '') + '\n-- \n' + s
}

// Replaces the old per-account free-text signature; migrate any that existed.
function migrateLegacySignature() {
  const legacy = localStorage.getItem('boi-sig-' + selectedAccount.value)
  if (legacy && legacy.trim() && signatures.value.length === 0) {
    signatures.value.push({ id: 'sig-' + Date.now(), name: 'Default', text: legacy })
    localStorage.removeItem('boi-sig-' + selectedAccount.value)
    setSigDefaultId(signatures.value[0].id)
    persistSignatures()
  }
}

function openSigManager() {
  loadSignatures(); migrateLegacySignature()
  startNewSig()
  settingsTab.value = 'signatures'
  openSettings()
}
function startNewSig() { sigEditId.value = ''; sigEditIsNew.value = true; sigEditName.value = ''; sigEditText.value = '' }
function startEditSig(s: SavedSignature) {
  sigEditId.value = s.id; sigEditIsNew.value = false; sigEditName.value = s.name; sigEditText.value = s.text
}
function saveSigEdit() {
  const name = sigEditName.value.trim()
  if (!name) return
  if (sigEditIsNew.value || !sigEditId.value) {
    const id = 'sig-' + Date.now()
    signatures.value.push({ id, name, text: sigEditText.value })
    if (signatures.value.length === 1) setSigDefaultId(id)
  } else {
    const s = sigById(sigEditId.value)
    if (s) { s.name = name; s.text = sigEditText.value }
  }
  persistSignatures(); loadSignatures()
}
function deleteSig(id: string) {
  signatures.value = signatures.value.filter(s => s.id !== id)
  if (sigDefaultId() === id) setSigDefaultId('')
  persistSignatures(); loadSignatures()
}
function setDefault(id: string) {
  setSigDefaultId(id)
  loadSignatures()
}


function openReply() {
  if (!selectedEmail.value) return
  // Prefill CC from the email's CC recipients (if any) so replies keep the
  // thread copied in — standard mail-client behaviour.
  openCompose(selectedEmail.value.fromEmail, 'Re: ' + selectedEmail.value.subject, '', selectedEmail.value.cc || '')
}

function openForward() {
  if (!selectedEmail.value) return
  openCompose('', 'Fwd: ' + selectedEmail.value.subject)
}

const downloadingRef = ref<string|null>(null)
const openingRef = ref<string|null>(null)
const attachmentError = ref('')
const downloadsRef = ref('')

// Window controls are provided by the OS/native titlebar (minimise, maximise,
// close), so no in-app buttons are needed. We only add an F11 fullscreen toggle
// since some WMs lack a native fullscreen button on the titlebar.
const isFullscreen = ref(false)
function toggleFullscreen() {
  isFullscreen.value = !isFullscreen.value
  if (isWailsNative()) {
    isFullscreen.value ? WailsFullscreen() : WailsUnfullscreen()
  } else {
    document.documentElement.requestFullscreen?.().catch(() => {})
      || document.exitFullscreen?.()
  }
}

async function downloadAttachment(a: EmailAttachment) {
  if (!selectedEmail.value) return
  downloadingRef.value = a.fileReference
  attachmentError.value = ''
  try {
    // WebKitGTK refuses a synthetic <a download> click, so the file is saved
    // server-side (in Go) to ~/Downloads and we just surface the path.
    const path = await wails.call('DownloadAttachment',
      selectedEmail.value.accountId, a.fileReference, a.displayName || '')
    if (!path) { downloadsRef.value = ''; return } // cancelled or no-op
    downloadsRef.value = path
  } catch (e: any) {
    attachmentError.value = e?.message || String(e)
  } finally {
    downloadingRef.value = null
  }
}

const FRAU_VIDEO = 'https://www.youtube.com/watch?v=nx8LjVmoSZk'

function openFrau() { wails.call('OpenExternal', FRAU_VIDEO) }

/* ============================================================
   DIARY — day / week / month calendar
   ============================================================ */
const diaryView = ref<'day' | 'week' | 'month'>('month')
// diaryCursor is the *focused* instant: exactly that day in Day view, the
// start of a week in Week view, the first of the month in Month view.
const diaryCursor = ref(new Date())
const diarySelKey = ref('')  // dmy key of the day selected in Month view

function openDiary() {
  diaryCursor.value = new Date()
  diarySelKey.value = dmy(new Date())
  showDiary.value = true
}

const dayIndex = (d: Date) => (d.getDay() + 6) % 7 // Monday-first
function startOfWeek(d: Date): Date {
  const c = new Date(d); c.setHours(0, 0, 0, 0)
  const back = dayIndex(c)
  c.setDate(c.getDate() - back)
  return c
}

function shiftDiary(dir: number) {
  const c = diaryCursor.value
  if (diaryView.value === 'day') {
    const n = new Date(c); n.setHours(12, 0, 0, 0); n.setDate(n.getDate() + dir)
    diaryCursor.value = n
  } else if (diaryView.value === 'week') {
    const n = startOfWeek(c); n.setDate(n.getDate() + dir * 7)
    diaryCursor.value = n
  } else {
    diaryCursor.value = new Date(c.getFullYear(), c.getMonth() + dir, 1)
    // keep the previously-selected day in view when possible
    if (diarySelKey.value) {
      const [d1, m1, y1] = diarySelKey.value.split('/').map(Number)
      if (y1 + 2000 === c.getFullYear() && m1 - 1 === c.getMonth()) {
        diarySelKey.value = dmy(new Date(c.getFullYear(), c.getMonth() + dir, Math.min(d1, new Date(c.getFullYear(), c.getMonth() + dir + 1, 0).getDate())))
      }
    }
  }
}

function todayDiary() {
  diaryCursor.value = new Date()
  diarySelKey.value = dmy(new Date())
}

const diaryTitle = computed(() => {
  const c = diaryCursor.value
  if (diaryView.value === 'day') return c.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
  if (diaryView.value === 'week') {
    const s = startOfWeek(c), e = new Date(s); e.setDate(e.getDate() + 6)
    const f = (x: Date) => x.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
    return s.getMonth() === e.getMonth() ? `${f(s)} – ${f(e)} ${e.toLocaleDateString('en-GB', { year: 'numeric' })}`
      : `${f(s)} – ${f(e)}`
  }
  return c.toLocaleDateString('en-GB', { month: 'long', year: 'numeric' })
})

// dayKey(date) -> "dd/mm/yy"
function dkey(d: Date): string {
  return dmy(new Date(d))
}
function evDayKey(ev: any): string {
  const d = new Date(ev.startTime); if (isNaN(d.getTime())) return ''
  return dmy(d)
}
// All the calendar-day keys an event occupies. A timed or all-day event that
// runs from startTime across endTime (e.g. a 3-day stay) must mark EVERY day
// it covers, not just its start day. endTime is exclusive in EAS/VCALENDAR,
// so the last day is the day before endTime (unless endTime is missing or
// earlier, in which case it's just the start day).
function evDayKeys(ev: any): string[] {
  const s = new Date(ev.startTime)
  if (isNaN(s.getTime())) return []
  s.setHours(0, 0, 0, 0)
  let e = ev.endTime ? new Date(ev.endTime) : null
  if (e && !isNaN(e.getTime())) {
    e.setHours(0, 0, 0, 0)
    // endTime is exclusive → last covered day is the day BEFORE it.
    e.setDate(e.getDate() - 1)
  } else {
    e = new Date(s)
  }
  if (e < s) e = new Date(s)
  const days: string[] = []
  const cursor = new Date(s)
  let guard = 0
  while (cursor <= e && guard < 4000) {
    days.push(dmy(cursor))
    cursor.setDate(cursor.getDate() + 1)
    guard++
  }
  return days
}
function isTodayDate(d: Date): boolean {
  const t = new Date()
  return d.getFullYear() === t.getFullYear() && d.getMonth() === t.getMonth() && d.getDate() === t.getDate()
}

// events on a specific calendar day (start falls within that day)
function evsOnDay(d: Date): any[] {
  const k = dkey(d)
  return calendarEvents.value
    .filter(ev => evDayKeys(ev).includes(k))
    .slice().sort((a, b) => new Date(a.startTime).getTime() - new Date(b.startTime).getTime())
}

// ---- Day view ----
const diaryDayEvents = computed(() => evsOnDay(diaryCursor.value))

// ---- Week view: 7 columns Monday-first ----
const diaryWeek = computed(() => {
  const s = startOfWeek(diaryCursor.value)
  const cols = []
  for (let i = 0; i < 7; i++) {
    const d = new Date(s); d.setDate(d.getDate() + i); d.setHours(12, 0, 0, 0)
    cols.push({ date: new Date(d), key: dkey(d), evs: evsOnDay(d) })
  }
  return cols
})

// ---- Month view: grid cells ----
const diaryCalendar = computed<{ day: number; inMonth: boolean; isToday: boolean; has: boolean; evs: any[]; key: string }[]>(() => {
  const m = diaryCursor.value
  const first = new Date(m.getFullYear(), m.getMonth(), 1)
  const startWeekday = dayIndex(first)
  const daysInMonth = new Date(m.getFullYear(), m.getMonth() + 1, 0).getDate()
  const prevDays = new Date(m.getFullYear(), m.getMonth(), 0).getDate()
  const byDate: Record<string, any[]> = {}
  for (const ev of calendarEvents.value) {
    for (const k of evDayKeys(ev)) (byDate[k] ||= []).push(ev)
  }
  const cells: { day: number; inMonth: boolean; isToday: boolean; has: boolean; evs: any[]; key: string }[] = []
  for (let i = 0; i < startWeekday; i++) {
    cells.push({ day: prevDays - startWeekday + i + 1, inMonth: false, isToday: false, has: false, evs: [], key: '' })
  }
  for (let d = 1; d <= daysInMonth; d++) {
    const k = dmy(new Date(m.getFullYear(), m.getMonth(), d))
    const evs = (byDate[k] || []).slice().sort((a, b) => new Date(a.startTime).getTime() - new Date(b.startTime).getTime())
    cells.push({ day: d, inMonth: true, isToday: isTodayDate(new Date(m.getFullYear(), m.getMonth(), d)), has: evs.length > 0, evs, key: k })
  }
  let trailing = 1
  while (cells.length % 7 !== 0) cells.push({ day: trailing++, inMonth: false, isToday: false, has: false, evs: [], key: '' })
  return cells
})

const diaryWeekdays = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const diarySelEvents = computed<any[]>(() =>
  diaryCalendar.value.find(c => c.inMonth && c.key === diarySelKey.value)?.evs || [])

function pickDiaryDay(c: any) {
  if (!c.inMonth) return
  diarySelKey.value = c.key
  // The highlighted day becomes the New-event target: move diaryCursor to it
  // so openNewEvent() (which builds its date from diaryCursor) creates the
  // entry on the day you picked rather than always today.
  const [dd, mm, yy] = c.key.split('/').map(Number)
  if (dd && mm) diaryCursor.value = new Date(2000 + yy, mm - 1, dd)
}

function fmtTime(t: any): string {
  if (!t) return ''
  const d = new Date(t); if (isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
}

function isAllDay(ev: any): boolean {
  if (ev.allDay) return true
  if (!ev.endTime) return true
  const s = new Date(ev.startTime), e = new Date(ev.endTime)
  if (e.getTime() - s.getTime() <= 0) return true
  return s.getHours() === 0 && s.getMinutes() === 0 && e.getHours() === 0 && e.getMinutes() === 0 && (e.getTime() - s.getTime()) >= 86400000
}

function timeRange(ev: any): string {
  if (isAllDay(ev)) return 'All day'
  const s = fmtTime(ev.startTime)
  const e = ev.endTime ? fmtTime(ev.endTime) : ''
  return e && e !== s ? `${s} – ${e}` : s
}


async function openAttachment(a: EmailAttachment) {
  if (!selectedEmail.value) return
  openingRef.value = a.fileReference
  attachmentError.value = ''; downloadsRef.value = ''
  try {
    // Fetched + handed to the system's default app (xdg-open) server-side.
    const path = await wails.call('OpenAttachment',
      selectedEmail.value.accountId, a.fileReference, a.displayName || '')
    if (path) {
      downloadsRef.value = `Opened with default app: ${path}`
    }
  } catch (e: any) {
    attachmentError.value = e?.message || String(e)
  } finally {
    openingRef.value = null
  }
}

// Print the open email: opens the message in a hidden print window (WebKit)
// and triggers the print dialog.
function printEmail() {
  const e = selectedEmail.value
  if (!e) return
  let bodyHtml = e.body || e.preview || ''
  if (e.bodyType !== 'html' || !bodyHtml.includes('<')) {
    bodyHtml = '<pre style="white-space:pre-wrap;font-family:inherit">' + (bodyHtml.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')) + '</pre>'
  }
  const header = `
    <div style="border-bottom:1px solid #ccc;padding-bottom:12px;margin-bottom:16px;font-family:system-ui,sans-serif;color:#222">
      <h2 style="margin:0 0 8px">${(e.subject || '(no subject)').replace(/</g, '&lt;')}</h2>
      <div style="font-size:13px;line-height:1.7">
        <div><strong>From:</strong> ${(e.from || '').replace(/</g, '&lt;')} ${(e.fromEmail ? '&lt;' + e.fromEmail + '&gt;' : '').replace(/</g, '&lt;')}</div>
        <div style="color:#666">${e.to ? 'To: ' + e.to.replace(/</g, '&lt;') : ''}</div>
        <div style="color:#666">${formatDate(e.dateReceived)} ${formatTime(e.dateReceived)}</div>
      </div>
    </div>`
  const title = (e.subject || 'message').replace(/</g, '&lt;')
  renderPrintFrame(`<!doctype html><html><head><meta charset="utf-8"><title>Print — ${title}</title></head>
    <body style="font-family:system-ui,sans-serif;color:#222">${header}${bodyHtml}</body></html>`)
}

// Print by writing to a hidden iframe in the current document and calling its
// print(). Wails/WebKitGTK blocks window.open() popups by default, so the old
// approach (new window) surfaced a "Pop-up blocked" alert instead of printing.
let __printFrame = null
function renderPrintFrame(doc: string) {
  if (!__printFrame) {
    __printFrame = document.createElement('iframe')
    __printFrame.setAttribute('style', 'position:fixed;width:0;height:0;border:0;visibility:hidden')
    __printFrame.setAttribute('aria-hidden', 'true')
    __printFrame.setAttribute('tabindex', '-1')
    document.body.appendChild(__printFrame)
  }
  const f = __printFrame
  const docWin = f.contentWindow
  if (!docWin) return
  docWin.document.open()
  docWin.document.write(doc)
  docWin.document.close()
  setTimeout(() => { try { docWin.print() } catch (err) { alert('Print failed: ' + err) } }, 300)
}

async function saveHTMLSource() {
  if (!selectedEmail.value) return
  attachmentError.value = ''; downloadsRef.value = ''
  try {
    const subject = (selectedEmail.value.subject || 'message').replace(/[^\w\- ]+/g, '').trim() || 'message'
    const path = await wails.call('SaveHTMLAs', selectedEmail.value.body || '', `${subject}.html`)
    if (!path) return // cancelled
    downloadsRef.value = path
  } catch (e: any) {
    attachmentError.value = e?.message || String(e)
  } finally {
    downloadingRef.value = null
  }
}

function fmtSize(n?: number): string {
  if (!n) return ''
  if (n < 1024) return n + ' B'
  if (n < 1048576) return (n/1024).toFixed(1) + ' KB'
  return (n/1048576).toFixed(1) + ' MB'
}

async function sendCompose() {
  if (!composeTo.value || !composeSubject.value) { composeError.value = 'To and Subject are required'; return }
  composeSending.value = true; composeError.value = ''
  try {
    // If they changed away from the default, remember that choice for this account.
    setSigDefaultId(composeSigId.value)
    await wails.call('SendMail', selectedAccount.value, composeTo.value, composeCc.value, composeBcc.value, composeSubject.value, signaturePreview())
    composeSuccess.value = 'Sent!'
    setTimeout(async () => {
      showCompose.value = false; composeSuccess.value = ''
      await syncAll()
    }, 1200)
  } catch (e: any) { composeError.value = e.message || String(e) }
  composeSending.value = false
}

async function addAccount() {
  addingAccount.value = true; addAccountError.value = ''
  try {
    const acc = await wails.call('AddAccount', newAccountName.value, newAccountEmail.value, newAccountPassword.value, newAccountServer.value)
    if (acc) {
      accounts.value = await wails.call('ListAccounts') || []
      selectedAccount.value = acc.id
      showAddAccount.value = false; newAccountName.value = ''; newAccountEmail.value = ''; newAccountPassword.value = ''
      await syncAll()
    }
  } catch (e: any) { addAccountError.value = e.message || String(e) }
  addingAccount.value = false
}

async function removeAccount() {
  if (!selectedAccount.value || !confirm('Remove this account?')) return
  try {
    await wails.call('RemoveAccount', selectedAccount.value)
    accounts.value = await wails.call('ListAccounts') || []
    selectedAccount.value = accounts.value.length > 0 ? accounts.value[0].id : ''
  } catch (e: any) { error.value = e.message }
}

// OOF
const oofSettings = ref({ state: 'disabled', internalReply: '', externalReply: '' })
const oofSaving = ref(false)
const oofMessage = ref('')

const appVersion = ref('…')
onMounted(async () => {
  // Register event handlers BEFORE the first sync: loadAccounts (called below)
  // triggers a SyncFolders that can race the Go connect goroutine; the
  // account-connected retry must not fire into an unregistered handler.
  wails.on('account-connected', (id: string) => { if (id === selectedAccount.value) syncAll() })
  loadAccounts()
  loadSignatures(); migrateLegacySignature()
  restoreSyncedFolders()
  try { appVersion.value = String(await wails.call('GetVersion')) } catch { appVersion.value = '' }
  // Background poll (Go startMailPolling) emits this only when a folder
  // actually changed (added/deleted mail). Reload just that folder and merge
  // it into the cache — the old handler called loadAllEmails() (all folders,
  // bodies included) every round even when nothing changed, freezing the UI.
  wails.on('emails-updated', (account: string, folderServerId: string) => {
    if (account !== selectedAccount.value) return
    const folder = folders.value.find(f => f.serverId === folderServerId)
    if (!folder) return
    reloadFolderEmails(folder)
  })
  window.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'F11') { e.preventDefault(); toggleFullscreen() }
  })
  // Email bodies are injected as raw HTML via v-html, so their <a href>
  // links are live anchors inside the app's own WebView. Without interception
  // a click navigates the whole Wails window to the external page (email view
  // vanishes, back/forward breaks) — especially jarring on macOS. Route
  // http(s) links to the system default browser via OpenExternal and stop the
  // in-app navigation. mailto: links get the same browser treatment.
  document.addEventListener('click', (e: MouseEvent) => {
    const target = e.target as HTMLElement | null
    const anchor = target && target.closest && target.closest('a[href]') as HTMLAnchorElement | null
    if (!anchor) return
    const href = anchor.getAttribute('href') || ''
    if (/^(https?:|mailto:)/i.test(href)) {
      e.preventDefault()
      e.stopPropagation()
      wails.call('OpenExternal', href)
        .catch(() => { window.location.assign(href) })
    }
  })
})
// Never start a sync while one is already running — two concurrent drains on
// the same folder make SOGo return 405 Not Allowed and the loser aborts
// mid-window, leaving the store stuck on old mail. syncAll() is the guarded
// entry point (see below); syncQueued lives here so both syncAll and the
// account-connected handler share it.
let syncQueued = false
watch(selectedAccount, () => { selectedPerson.value = null; selectedEmail.value = null; allEmails.value = []; if (selectedAccount.value) syncAll() })
</script>

<template>
  <!-- ===== NO ACCOUNTS: SETUP ===== -->
  <div class="setup" v-if="accounts.length === 0">
    <div class="setup-card">
      <div class="setup-logo"><img src="/boi-icon.png" alt="BOI" /></div>
      <p class="muted">Connect your email account to get started</p>
      <div v-if="addAccountError" class="error-banner">{{ addAccountError }}</div>
      <div class="form-group"><label>Your Name</label><input v-model="newAccountName" placeholder="Rupert Chandler" /></div>
      <div class="form-group"><label>Email</label><input v-model="newAccountEmail" placeholder="you@example.com" /></div>
      <div class="form-group"><label>Password</label><input type="password" v-model="newAccountPassword" placeholder="App password" /></div>
      <div class="form-group"><label>Server</label><input v-model="newAccountServer" /></div>
      <button class="primary-btn" @click="addAccount" :disabled="addingAccount" style="width:100%">{{ addingAccount ? 'Connecting...' : 'Add Account' }}</button>
    </div>
  </div>

  <!-- ===== MAIN APP: UNIBOX PEOPLE-CENTRIC ===== -->
  <div class="app" v-else>
    <!-- ===== LEFT: People list ===== -->
    <div class="left">
      <div class="left-toolbar">
        <div class="toolbar-row">
          <a href="#" class="logo-link" @click.prevent="openFrau()" title="Frau Blücher—the horses know what that means"><img src="/boi-icon.png" class="logo" alt="BOI" /></a>
          <select v-model="selectedAccount" class="account-sel">
            <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.email }}</option>
          </select>
          <span class="account-dot-wrap"><span v-for="a in accounts" :key="a.id" class="account-dot" :style="{ background: accountColour(a.email) }"></span></span>
          <button class="icon-btn" @click="syncAll" :disabled="syncing" :title="syncing ? 'Syncing...' : 'Sync'"><span :class="{ spin: syncing }">🔄</span></button>
        </div>
      </div>

      <div class="search-row">
        <input v-model="searchQuery" placeholder="Search people or subjects..." class="search-input" />
        <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''" title="Clear search">✕</button>
      </div>

      <!-- Compose row: contacts + calendar + write, all the same size -->
      <div class="compose-row">
        <button class="compose-action" @click="showContacts = true" title="Contacts">👥 Contacts</button>
        <button class="compose-action" @click="openDiary()" title="Diary">📅 Calendar</button>
        <button class="write-btn" @click="openCompose()" title="Compose a new message">✏️ Write</button>
      </div>

      <div class="sort-row">
        <button :class="['sort-pill', { active: sortBy === 'date' }]" @click="sortBy = 'date'">Date</button>
        <button :class="['sort-pill', { active: sortBy === 'name' }]" @click="sortBy = 'name'">Name</button>
        <button class="sort-pill sort-dir" :title="sortLabel" @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'">
          {{ sortBy === 'name' ? (sortDir === 'asc' ? 'A→Z' : 'Z→A') : (sortDir === 'desc' ? 'New→Old' : 'Old→New') }}
        </button>
      </div>

      <!-- People list: grouped by recency when date-sorted, flat A–Z when name-sorted -->
      <div class="sender-scroll">
        <template v-if="sortBy === 'date'">
          <template v-for="g in senderGroups" :key="g.label">
            <div class="group-label">{{ g.label }}</div>
            <button v-for="s in g.people" :key="s.email" :class="['sender-row', { active: selectedPerson === s.email }]" @click="selectPerson(s.email)">
              <img :src="avatarUrl(s.email, 36)" class="avatar-sm" />
              <div class="sender-info">
                <span class="sender-name">{{ s.name }}</span>
                <span class="sender-sub">{{ s.email }}</span>
              </div>
              <div class="sender-right">
                <span v-if="s.hasUnread" class="dot"></span>
                <span class="sender-ct">{{ s.count }}</span>
              </div>
            </button>
          </template>
        </template>

        <template v-else>
          <button v-for="s in sortedPeople" :key="s.email" :class="['sender-row', { active: selectedPerson === s.email }]" @click="selectPerson(s.email)">
            <img :src="avatarUrl(s.email, 36)" class="avatar-sm" />
            <div class="sender-info">
              <span class="sender-name">{{ s.name }}</span>
              <span class="sender-sub">{{ s.email }}</span>
            </div>
            <div class="sender-right">
              <span v-if="s.hasUnread" class="dot"></span>
              <span class="sender-ct">{{ s.count }}</span>
            </div>
          </button>
        </template>

        <div v-if="sortedPeople.length === 0" class="empty-hint">{{ syncing ? 'Syncing...' : 'No emails yet — tap 🔄 to sync' }}</div>
      </div>

      <!-- Settings gear + Source viewer + build number: tucked under the people list -->
      <div class="left-footer">
        <button class="icon-btn gear-btn" @click="openActivity()" title="Activity log (received/sent + server comms)">🕘</button>
        <button class="icon-btn gear-btn" @click="openSource()" title="View message source & header explanation">📄</button>
        <button class="icon-btn gear-btn" @click="openSettingsAccounts()" title="Settings">⚙</button>
        <span class="left-version" title="BOI build">v{{ appVersion }}</span>
      </div>
    </div>

    <!-- ===== RIGHT: Conversation / Reading ===== -->
    <div class="right">
      <div v-if="error" class="error-banner" @click="error = ''">{{ error }}</div>

      <!-- No person selected -->
      <div v-if="!selectedPerson" class="welcome">
        <div class="welcome-icon">📨</div>
        <h2>Select a person to see your conversation</h2>
        <p class="muted">Sent and received emails grouped by person, like a chat</p>
      </div>

      <!-- Person selected: chat-style email list -->
      <div v-else-if="!selectedEmail" class="conversation">
        <div class="conv-header">
          <button class="back-btn" @click="backToPeople" title="Back to people">← People</button>
          <img :src="avatarUrl(selectedPerson, 44)" class="avatar-md" />
          <div class="conv-sender-info">
            <h2>{{ peopleGroups.find(p => p.email === selectedPerson)?.name || selectedPerson }}</h2>
            <span class="muted">{{ selectedPerson }}</span>
          </div>
          <button class="action-btn" @click="openCompose(selectedPerson)" title="Write to this person">✏️ Write</button>
        </div>

        <!-- Selection toolbar: appears once emails are multi-selected -->
        <div v-if="selectedEmailIds.length > 0" class="sel-toolbar">
          <span class="sel-count">{{ selectedEmailIds.length }} selected</span>
          <select class="folder-sel sel-target" @change="moveSelectedGroup(($event.target as HTMLSelectElement).value); ($event.target as HTMLSelectElement).value = ''">
            <option value="">Move to folder…</option>
            <option v-for="f in moveDestFolders" :key="f.id" :value="f.id">{{ f.name }}</option>
          </select>
          <button class="action-btn danger-btn" @click="deleteSelectedGroup()" title="Move selected to Trash">🗑 Delete</button>
          <button class="action-btn muted" @click="selectedEmailIds = []" title="Clear selection">✕ Clear</button>
        </div>

        <div class="conv-scroll">
          <div v-for="e in personEmails" :key="e.id"
               :class="['bubble-row', isFromMe(e) ? 'bubble-sent' : 'bubble-received', { unread: !e.isRead, selected: selectedEmailIds.includes(e.id) }]"
               @click="onBubbleClick(e, $event)">
            <div class="bubble-avatar-col" v-if="!isFromMe(e)">
              <img :src="avatarUrl(e.fromEmail || e.from, 32)" class="avatar-xs" />
            </div>
            <div class="bubble-content" :style="isFromMe(e) ? '' : 'border-left-color:' + accountColour(accountEmail(e.accountId))">
              <div class="bubble-top-row">
                <span class="bubble-from">{{ isFromMe(e) ? 'You' : e.from }}</span>
                <span class="bubble-folder" v-if="isFromMe(e)">{{ folders.find(f => f.id === e.folderId)?.name }}</span>
                <span class="bubble-date">{{ formatShort(e.dateReceived) }}</span>
              </div>
              <div class="bubble-subject">{{ e.subject || '(no subject)' }}</div>
              <div class="bubble-preview">{{ strippreview(e.preview || e.body).substring(0, 140) }}</div>
              <div v-if="e.hasAttachment" class="bubble-attach">📎</div>
            </div>
            <div class="bubble-avatar-col" v-if="isFromMe(e)">
              <span class="bubble-you">You</span>
            </div>
            <!-- Per-email quick actions (visible on hover) -->
            <div class="bubble-actions" @click.stop>
              <button class="bubble-act" title="Move this email" @click.stop>
                📁
                <select class="bubble-move-sel" @change="moveListEmail(e, $event.target as HTMLSelectElement)">
                  <option value="" selected>Move</option>
                  <option v-for="f in moveDestFolders" :key="f.id" :value="f.serverId">{{ f.name }}</option>
                </select>
              </button>
              <button class="bubble-act danger-hover" title="Delete this email" @click.stop="deleteListEmail(e)">🗑</button>
            </div>
          </div>
          <div v-if="personEmails.length === 0" class="empty-hint">No emails with this person</div>
        </div>
      </div>

      <!-- Email selected: reading pane -->
      <div v-else class="reading">
        <div class="reading-header">
          <button class="back-btn" @click="backToConversation" title="Back to conversation">← Back</button>
          <img :src="avatarUrl(selectedEmail.fromEmail, 40)" class="avatar-sm" />
          <div class="reading-from">
            <strong>{{ selectedEmail.from }}</strong>
            <span class="muted">&lt;{{ selectedEmail.fromEmail }}&gt;</span>
          </div>
          <span class="reading-date">{{ formatDate(selectedEmail.dateReceived) }} {{ formatTime(selectedEmail.dateReceived) }}</span>
          <div class="reading-actions">
            <button class="action-btn" @click="openReply">↩ Reply</button>
            <button class="action-btn muted" @click="openForward">↗ Forward</button>
            <button class="action-btn muted" @click="printEmail" title="Print this email">🖨 Print</button>
            <button class="action-btn muted" v-if="selectedEmail.body && (selectedEmail.bodyType === 'html' || selectedEmail.body.includes('<'))"
              @click="saveHTMLSource">⬇ Save</button>
            <span class="read-actions-sep"></span>
            <button class="action-btn muted" @click="moveMenuOpen = !moveMenuOpen" title="Move this email to a folder">📁 Move</button>
            <button class="action-btn danger-btn" @click="deleteSingleEmail()" title="Move this email to Trash">🗑 Delete</button>
          </div>
          <div v-if="moveMenuOpen" class="move-menu" @click.stop>
            <div class="move-menu-title muted">Move to folder</div>
            <button v-for="f in moveDestFolders" :key="f.id" class="move-menu-item" @click="moveSingleEmail(f.serverId)">{{ f.name }}</button>
          </div>
        </div>
        <div class="reading-subject">{{ selectedEmail.subject }}</div>
        <div class="reading-to muted">To: {{ selectedEmail.to }} &lt;{{ selectedEmail.toEmails?.join(', ') }}&gt;</div>
        <div class="reading-body-card">
          <div class="reading-body" v-if="selectedEmail.bodyType === 'html' || (selectedEmail.body && selectedEmail.body.includes('<'))" v-html="selectedEmail.body"></div>
          <div class="reading-body" v-else>
            <p v-for="(para, i) in (selectedEmail.body || selectedEmail.preview || 'No content').split(/\n{2,}/)" :key="i">{{ para.trim() }}</p>
          </div>
        </div>
        <div v-if="selectedEmail.hasAttachment" class="attachment-bar">
          <span class="attachment-bar-title">📎 Attachments</span>
          <p v-if="attachmentError" class="error-text">{{ attachmentError }}</p>
          <p v-if="downloadsRef" class="success-msg">Saved to {{ downloadsRef }}</p>
          <div v-if="selectedEmail.attachments?.length" class="attachment-chips">
            <div v-for="a in selectedEmail.attachments" :key="a.fileReference" class="attachment-chip">
              <span class="chip-icon">📄</span>
              <div class="chip-meta">
                <span class="chip-name">{{ a.displayName }}</span>
                <span v-if="a.estimatedDataSize" class="chip-size muted">{{ fmtSize(a.estimatedDataSize) }}</span>
              </div>
              <button class="chip-dl" @click="openAttachment(a)" :disabled="openingRef === a.fileReference" :title="'Open with default app'">
                {{ openingRef === a.fileReference ? '…' : '▶' }}
              </button>
              <button class="chip-dl" @click="downloadAttachment(a)" :disabled="downloadingRef === a.fileReference" :title="'Save to file'">
                {{ downloadingRef === a.fileReference ? '…' : '⬇' }}
              </button>
            </div>
          </div>
          <span v-else class="muted attachment-empty">(no file references on sync response)</span>
        </div>
      </div>
    </div>
  </div>

  <!-- ===== COMPOSE MODAL ===== -->
  <div v-if="showCompose" class="modal-overlay" @click.self.prevent>
    <div class="modal">
      <div class="modal-header">
        <h3>✏️ Compose</h3>
        <button class="icon-btn" @click="showCompose = false">✕</button>
      </div>
      <div class="form-group"><label>To</label><input ref="composeToEl" v-model="composeTo" placeholder="recipient@example.com" autocomplete="off" @focus="showComposeSug = true" @input="showComposeSug = true" @blur="onComposeToBlur" /></div>
      <div v-if="showComposeSug && composeSuggestions.length" class="compose-suggest">
        <button v-for="s in composeSuggestions" :key="s.address" class="suggest-item" @mousedown.prevent="pickSuggestion(s)">
          <span class="suggest-addr">{{ s.address }}</span>
          <span class="suggest-name" v-if="s.label !== s.address">{{ s.label }}</span>
        </button>
      </div>
      <div v-if="showCcBcc">
        <div class="form-group"><label>Cc</label><input v-model="composeCc" placeholder="cc@example.com, cc2@example.com" autocomplete="off" /></div>
        <div class="form-group"><label>Bcc</label><input v-model="composeBcc" placeholder="bcc@example.com" autocomplete="off" /></div>
      </div>
      <div class="form-group"><label>Subject</label><input v-model="composeSubject" placeholder="Subject" /></div>
      <div class="form-group"><label>Message</label><textarea v-model="composeBody" rows="8" placeholder="Write your message..."></textarea></div>
      <div class="form-group">
        <label>Signature</label>
        <select v-model="composeSigId" class="account-sel">
          <option value="">No signature</option>
          <option v-for="s in signatures" :key="s.id" :value="s.id">{{ s.name }}{{ s.id === sigDefaultId() ? ' (default)' : '' }}</option>
        </select>
        <button class="icon-btn" @click="openSigManager()" title="Manage signatures" style="margin-left:8px;vertical-align:middle">⚙</button>
        <div class="sig-preview" v-if="sigById(composeSigId)?.text">{{ sigById(composeSigId)!.text }}</div>
      </div>
      <p v-if="composeError" class="error-text">{{ composeError }}</p>
      <p v-if="composeSuccess" class="success-msg">{{ composeSuccess }}</p>
      <div class="modal-actions">
        <button class="action-btn muted" @click="showCcBcc = !showCcBcc">{{ showCcBcc ? '− Cc/Bcc' : '+ Cc/Bcc' }}</button>
        <button class="primary-btn" @click="sendCompose" :disabled="composeSending">{{ composeSending ? 'Sending...' : 'Send' }}</button>
        <button class="action-btn muted" @click="showCompose = false">Cancel</button>
      </div>
    </div>
  </div>

  <!-- ===== CONTACTS VIEW ===== -->
  <div v-if="showContacts" class="modal-overlay" @click.self="showContacts = false">
    <div class="modal contacts-modal">
      <div class="modal-header">
        <h3>👥 Contacts ({{ contacts.length }})</h3>
        <button class="icon-btn" @click="showContacts = false">✕</button>
      </div>
      <div v-if="contacts.length === 0" class="empty-hint">{{ syncing ? 'Syncing…' : 'No contacts yet — EAS contacts and remembered addresses will appear here.' }}</div>
      <div v-else class="contact-list">
        <button v-for="c in contacts" :key="c.id" class="contact-row" @click="openCompose(c.email)">
          <img :src="avatarUrl(c.email, 36)" class="avatar-sm" />
          <div class="contact-meta">
            <span class="contact-name">{{ c.name || c.email }}</span>
            <span class="muted contact-email">{{ c.email }}</span>
          </div>
          <span class="contact-actions">✏️</span>
        </button>
      </div>
    </div>
  </div>

  <!-- ===== DIARY (CALENDAR) VIEW ===== -->
  <div v-if="showDiary" class="modal-overlay" @click.self="showDiary = false">
    <div class="modal diary-modal">
      <div class="modal-header">
        <h3>📅 Diary</h3>
        <div class="modal-header-actions">
          <button class="action-btn" @click="openNewEvent()" :disabled="!calendarFolder">＋ New</button>
          <button class="icon-btn" @click="showDiary = false">✕</button>
        </div>
      </div>
      <!-- Toolbar: view switcher + nav + Today -->
      <div class="diary-toolbar">
        <div class="diary-viewswitch">
          <button :class="['view-btn', { active: diaryView === 'day' }]" @click="diaryView = 'day'">Day</button>
          <button :class="['view-btn', { active: diaryView === 'week' }]" @click="diaryView = 'week'">Week</button>
          <button :class="['view-btn', { active: diaryView === 'month' }]" @click="diaryView = 'month'">Month</button>
        </div>
        <div class="diary-nav">
          <button class="icon-btn" @click="shiftDiary(-1)" title="Previous">‹</button>
          <button class="action-btn muted" @click="todayDiary()">Today</button>
          <button class="icon-btn" @click="shiftDiary(1)" title="Next">›</button>
        </div>
        <span class="diary-title">{{ diaryTitle }}</span>
      </div>

      <!-- ===== DAY VIEW ===== -->
      <div v-if="diaryView === 'day'" class="diary-dayview">
        <div v-if="!diaryDayEvents.length" class="empty-hint diary-empty">
          <span style="font-size:26px">🗓️</span>
          <span>{{ syncing ? 'Syncing…' : 'No events this day.' }}</span>
        </div>
        <div v-else class="diary-list">
          <div v-for="ev in diaryDayEvents" :key="ev.id" class="diary-card">
            <div class="diary-card-time" :class="{ 'allday': isAllDay(ev) }">
              <span class="diary-time-main">{{ timeRange(ev) }}</span>
              <span v-if="ev.serverId" class="diary-sync" title="Synced with server">synced</span>
            </div>
            <div class="diary-card-line"></div>
            <div class="diary-body">
              <span class="diary-subject">{{ ev.subject || '(no subject)' }}</span>
              <div class="diary-meta">
                <span v-if="ev.location" class="diary-chip" title="Location">📍 {{ ev.location }}</span>
                <span v-if="ev.attendees && ev.attendees.length" class="diary-chip" title="Attendees">👥 {{ ev.attendees.length }} person{{ ev.attendees.length === 1 ? '' : 's' }}</span>
                <span v-if="ev.organizerName" class="diary-chip" title="Organizer">👑 {{ ev.organizerName }}</span>
              </div>
            </div>
            <div class="diary-actions">
              <button class="icon-btn" title="Edit" @click="editEvent(ev)">✎</button>
              <button class="icon-btn danger" title="Delete" @click="removeEvent(ev)">🗑</button>
            </div>
          </div>
        </div>
      </div>

      <!-- ===== WEEK VIEW: 7 columns ===== -->
      <div v-else-if="diaryView === 'week'" class="diary-week">
        <div v-for="col in diaryWeek" :key="col.key" class="diary-weekday" :class="{ today: isTodayDate(col.date) }">
          <div class="diary-weekday-head">
            <span class="diary-weekday-name">{{ col.date.toLocaleDateString('en-GB', { weekday: 'short' }) }}</span>
            <span class="diary-weekday-num" :class="{ 'pill-today': isTodayDate(col.date) }">{{ col.date.getDate() }}</span>
          </div>
          <div class="diary-weekday-body">
            <div v-for="ev in col.evs" :key="ev.id" class="diary-wev" @dblclick="editEvent(ev)" :title="(timeRange(ev)) + ' — ' + (ev.subject || '(no subject)')">
              <span v-if="!isAllDay(ev)" class="diary-wtime">{{ fmtTime(ev.startTime) }}</span>
              <span class="diary-wsubj">{{ ev.subject || '(no subject)' }}</span>
            </div>
            <span v-if="!col.evs.length" class="diary-weekday-empty">.</span>
          </div>
        </div>
      </div>

      <!-- ===== MONTH VIEW: grid + selected-day agenda ===== -->
      <div v-else class="diary-monthview">
        <div class="diary-grid-head">
          <span v-for="w in diaryWeekdays" :key="w" class="diary-grid-cell diary-grid-wd">{{ w }}</span>
        </div>
        <div class="diary-grid">
          <button v-for="(c, i) in diaryCalendar" :key="i" class="diary-grid-cell"
            :class="{ 'out': !c.inMonth, 'today': c.isToday, 'sel': c.key === diarySelKey, 'has': c.has }"
            @click="pickDiaryDay(c)">
            <span class="diary-cell-day">{{ c.day }}</span>
            <div class="diary-cell-evs">
              <span v-for="ev in c.evs.slice(0, 3)" :key="ev.id" class="diary-cell-ev" :title="(timeRange(ev)) + ' — ' + (ev.subject || '(no subject)')">{{ ev.subject || '(no subject)' }}</span>
              <span v-if="c.evs.length > 3" class="diary-cell-more">+{{ c.evs.length - 3 }} more</span>
            </div>
          </button>
        </div>
        <div v-if="diarySelKey" class="diary-selday">
          <div class="diary-selday-head">
            <span class="diary-selday-label">
              {{ diarySelKey.split('/')[0] }} {{ diaryCursor.toLocaleDateString('en-GB', { month: 'long' }) }}
            </span>
          </div>
          <div v-if="!diarySelEvents.length" class="empty-hint" style="padding:12px">No events on this day.</div>
          <div v-else class="diary-list">
            <div v-for="ev in diarySelEvents" :key="ev.id" class="diary-card">
              <div class="diary-card-time" :class="{ 'allday': isAllDay(ev) }">
                <span class="diary-time-main">{{ timeRange(ev) }}</span>
              </div>
              <div class="diary-card-line"></div>
              <div class="diary-body">
                <span class="diary-subject">{{ ev.subject || '(no subject)' }}</span>
                <div class="diary-meta">
                  <span v-if="ev.location" class="diary-chip" title="Location">📍 {{ ev.location }}</span>
                </div>
              </div>
              <div class="diary-actions">
                <button class="icon-btn" title="Edit" @click="editEvent(ev)">✎</button>
                <button class="icon-btn danger" title="Delete" @click="removeEvent(ev)">🗑</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- ===== NEW EVENT MODAL ===== -->
  <div v-if="showNewEvent" class="modal-overlay" @click.self="closeNewEvent()">
    <div class="modal new-event-modal">
      <div class="modal-header">
        <h3>{{ editingEvent ? '✎ Edit Event' : '📅 New Event' }}</h3>
        <button class="icon-btn" @click="closeNewEvent()">✕</button>
      </div>
      <div class="modal-body">
        <div v-if="newEventError" class="error-banner">{{ newEventError }}</div>
        <div class="form-group"><label>Subject *</label><input v-model="newEventSubject" placeholder="What's happening?" /></div>
        <div class="form-group"><label>Location</label><input v-model="newEventLocation" placeholder="Where?" /></div>
        <div class="form-group"><label>Invitees (comma-separated emails)</label><input v-model="newEventInvitees" placeholder="bob@example.com, sue@example.com" /></div>
        <div class="form-group row-fields">
          <div>
            <label>Start * (dd/mm/yy)</label>
            <div class="dt-fields">
              <input v-model="newEventStartDate" placeholder="14/10/26" class="dt-date" />
              <input v-model="newEventStartTime" placeholder="09:30" class="dt-time" />
            </div>
          </div>
          <div>
            <label>End * (dd/mm/yy)</label>
            <div class="dt-fields">
              <input v-model="newEventEndDate" placeholder="14/10/26" class="dt-date" />
              <input v-model="newEventEndTime" placeholder="10:30" class="dt-time" />
            </div>
          </div>
        </div>
        <div class="form-group"><label class="check-label"><input type="checkbox" v-model="newEventAllDay" /> All day</label></div>
        <div class="modal-actions">
          <button class="action-btn" @click="closeNewEvent()">Cancel</button>
          <button v-if="editingEvent" class="action-btn danger-btn" @click="deleteEditingEvent()" :disabled="newEventSaving">Delete</button>
          <button class="action-btn primary-btn" @click="saveNewEvent()" :disabled="newEventSaving">{{ newEventSaving ? (editingEvent ? 'Saving…' : 'Creating…') : (editingEvent ? 'Save Changes' : 'Create Event') }}</button>
        </div>
      </div>
    </div>
  </div>

  <!-- ===== SETTINGS PAGE ===== -->
  <!-- ==== MESSAGE SOURCE & HEADER EXPLAINER ==== -->
  <div v-if="showSource" class="modal-overlay" @click.self="closeSource()">
    <div class="modal source-modal">
      <div class="modal-header">
        <h3>📄 Message source &amp; headers</h3>
        <button class="icon-btn" @click="closeSource()">✕</button>
      </div>

      <div v-if="!selectedEmail" class="empty-hint" style="padding:24px">
        Open an email first — its source and headers will appear here.
      </div>
      <template v-else>
        <div class="source-scroll">
          <div class="folder-panel-title" style="margin-bottom:6px">Raw source <span v-if="sourceLoading" class="muted" style="font-weight:400">(fetching from server…)</span></div>
          <pre v-if="sourceLoading && !sourceRaw" class="source-pre">Loading…</pre>
          <pre v-else class="source-pre">{{ sourceRaw || currentSourceText() }}</pre>
          <p v-if="sourceError" class="error-text" style="margin-top:8px">{{ sourceError }}</p>

          <div class="folder-panel-title" style="margin:16px 0 6px">What the headers mean</div>
          <div v-if="sourceLoading && parsedSourceHeaders.length === 0" class="muted" style="font-size:13px;padding:4px 0">Translating live headers…</div>
          <div v-else class="hdr-explain">
            <div v-if="parsedSourceHeaders.length">
              <div v-for="(h, i) in parsedSourceHeaders" :key="i" class="hdr-row">
                <code class="hdr-name" :title="h.value">{{ h.name }}</code>
                <span class="hdr-desc">{{ hdrName(h.name) }}</span>
              </div>
            </div>
            <div v-else>
              <div v-for="h in explainableHeaders" :key="h.name" class="hdr-row">
                <code class="hdr-name">{{ h.name }}</code>
                <span class="hdr-desc">{{ h.desc }}</span>
              </div>
            </div>
          </div>

          <p class="muted" style="font-size:12px;margin-top:14px">
            {{ sourceRaw ? 'Headers read live off the server and translated above — hover a name to see its exact value.' : 'Could not fetch live source, so this shows the standard breakdown instead.' }}
          </p>
        </div>
      </template>
    </div>
  </div>

  <!-- ===== ACTIVITY LOG VIEWER ===== -->
  <div v-if="showActivity" class="modal-overlay" @click.self="closeActivity()">
    <div class="modal activity-modal">
      <div class="modal-header">
        <h3>🕘 Activity log</h3>
        <button class="icon-btn" @click="closeActivity()">✕</button>
      </div>
      <div class="activity-meta muted" style="margin-bottom:8px">
        Received (RECV) and sent (SEND) mail with the server comms behind each —
        EAS sync trigger for incoming, SMTP session summary for outgoing.
      </div>
      <p v-if="activityError" class="error-text">{{ activityError }}</p>
      <p v-else-if="activityLoading && activityLines.length === 0" class="empty-hint">Loading…</p>
      <p v-else-if="activityLines.length === 0" class="empty-hint">No activity yet.</p>
      <div v-else class="activity-scroll">
        <div v-for="(line, i) in activityLines" :key="i" class="activity-line">
          <span class="activity-kind" :class="line.includes('RECV') ? 'k-recv' : line.includes('SEND') ? 'k-send' : 'k-other'">
            {{ line.includes('RECV') ? '▾ RECV' : line.includes('SEND') ? '▴ SEND' : '·' }}
          </span>
          <span class="activity-text">{{ line }}</span>
        </div>
      </div>
      <div class="modal-actions" style="justify-content:space-between;margin-top:10px">
        <span class="muted" style="font-size:12px">log: ~/.config/boi/activity.log</span>
        <button class="action-btn" @click="openActivity()" :disabled="activityLoading">↻ Refresh</button>
      </div>
    </div>
  </div>

  <div v-if="showSettings" class="modal-overlay" @click.self="closeSettings()">
    <div class="modal settings">
      <div class="modal-header">
        <h3>⚙ Settings</h3>
        <button class="icon-btn" @click="closeSettings()">✕</button>
      </div>

      <div class="settings-tabs">
        <button :class="['settings-tab', { active: settingsTab === 'accounts' }]" @click="settingsTab = 'accounts'; loadAccounts()">Accounts</button>
        <button :class="['settings-tab', { active: settingsTab === 'sync' }]" @click="settingsTab = 'sync'">Mail &amp; Folders</button>
        <button :class="['settings-tab', { active: settingsTab === 'signatures' }]" @click="settingsTab = 'signatures'; loadSignatures(); startNewSig()">Signatures</button>
      </div>

      <!-- ==== ACCOUNTS TAB ==== -->
      <div v-if="settingsTab === 'accounts'" class="settings-pane">
        <div v-if="addAccountError" class="error-banner">{{ addAccountError }}</div>
        <div class="settings-cols">
          <!-- left: colour-coded account list -->
          <div class="settings-col">
            <div class="folder-panel-title" style="margin-bottom:6px">Accounts</div>
            <div class="acc-list">
              <div v-for="a in accounts" :key="a.id" class="acc-item">
                <span class="account-dot" :style="{ background: accountColour(a.email) }"></span>
                <div class="acc-item-main">
                  <span class="acc-item-name">{{ a.name || a.email }}</span>
                  <span class="acc-item-email">{{ a.email }}</span>
                  <span v-if="a.id === selectedAccount" class="acc-item-current">current</span>
                  <span v-if="a.connected" class="acc-item-connected">● connected</span>
                </div>
                <button class="icon-btn" @click="startAccountEditor(a)" title="Edit account">✏️</button>
                <button class="icon-btn" @click="deleteAccount(a)" title="Remove account">🗑</button>
              </div>
            </div>
            <p class="muted" style="font-size:12px;margin-top:8px">Colours are assigned automatically per mailbox, so the same account looks the same on every machine.</p>
          </div>
          <!-- right: editor -->
          <div class="settings-col">
            <div class="folder-panel-title" style="margin-bottom:6px">{{ editingAccountId ? 'Edit account' : 'Add account' }}</div>
            <div class="form-group"><label>Name</label>
              <input v-model="newAccountName" placeholder="Your name (e.g. Rupert Chandler)" />
            </div>
            <div class="form-group"><label>Email</label><input v-model="newAccountEmail" placeholder="you@example.com" /></div>
            <div class="form-group"><label>Password</label><input type="password" v-model="newAccountPassword" :placeholder="editingAccountId ? 'Leave blank to keep current' : 'App password'" /></div>
            <div class="form-group"><label>Server</label><input v-model="newAccountServer" /></div>
            <div class="modal-actions" style="justify-content:flex-start">
              <button class="primary-btn" @click="saveAccount" :disabled="addingAccount">{{ addingAccount ? 'Saving...' : (editingAccountId ? 'Save Changes' : 'Add Account') }}</button>
              <button class="action-btn muted" v-if="editingAccountId" @click="startNewAccount()">Cancel Edit</button>
            </div>
          </div>
        </div>
      </div>

      <!-- ==== SYNC / MAIL & FOLDERS TAB ==== -->
      <div v-if="settingsTab === 'sync'" class="settings-pane">
        <div class="settings-cols">
          <!-- left column: what's synced + viewed mailbox -->
          <div class="settings-col">
            <div class="folder-panel-title" style="margin-bottom:6px">Sync folders</div>
            <p class="muted" style="font-size:12px;margin-bottom:10px">Choose which mail folders BOI keeps in sync. Mail in unselected folders stays on the server and won't appear in BOI.</p>
            <div class="folder-panel-sync" style="border:none;background:transparent;padding:0">
              <template v-for="item in visibleFolderTree" :key="item.f.id">
                <label :class="['folder-check', { 'folder-indent': item.depth > 0 }]" :style="{ paddingLeft: (8 + item.depth * 16) + 'px' }">
                  <button v-if="folderHasChildren(item.f)" class="folder-twisty" @click.prevent="toggleFolder(item.f)">{{ folderCollapsed.has(item.f.id) ? '▸' : '▾' }}</button>
                  <span v-else class="folder-twisty folder-twisty-spacer">·</span>
                  <input type="checkbox" :checked="syncedFolderIds.includes(item.f.id)" @change="toggleFolderSync(item.f.id)" />
                  <span class="folder-icon">{{ folderIcon(item.f) }}</span>
                  <span class="folder-name">{{ item.f.name }}</span>
                  <span v-if="item.f.unreadCount" class="folder-unread">{{ item.f.unreadCount }}</span>
                </label>
              </template>
            </div>

            <div class="folder-panel-title" style="margin:18px 0 6px">Viewed mailbox</div>
            <p class="muted" style="font-size:12px;margin-bottom:8px">Filter the people list to a single folder, or show all synced mail.</p>
            <div class="folder-row" style="padding:0">
              <button :class="['pill', { active: !selectedFolder }]" @click="clearFolder()">All Mail</button>
              <select v-model="selectedFolder" class="folder-sel" @change="selectedPerson = null; selectedEmail = null">
                <option value="">All synced</option>
                <option v-for="item in indentedFolders.filter(i => syncedFolderIds.includes(i.f.id))" :key="item.f.id" :value="item.f.id">{{ '　'.repeat(item.depth) }}{{ item.f.name }}</option>
              </select>
            </div>
            <div class="modal-actions" style="justify-content:flex-end;margin-top:10px">
              <button class="primary-btn" @click="syncAll">Sync now</button>
            </div>
          </div>
          <!-- right column: create folder + folder manager -->
          <div class="settings-col">
            <div class="folder-panel-title" style="margin-bottom:6px">New folder</div>
            <p class="muted" style="font-size:12px;margin-bottom:8px">Create a folder on the server to file mail into. Pick a parent to make it a subfolder.</p>
            <div class="mailman-row">
              <select v-model="newFolderParentId" class="folder-sel" style="width:38%">
                <option value="">Mailbox root</option>
                <option v-for="item in indentedFolders" :key="item.f.id" :value="item.f.id">{{ '　'.repeat(item.depth) }}{{ item.f.name }}</option>
              </select>
              <input v-model="newFolderName" class="folder-sel" style="flex:1" placeholder="e.g. Invoices" @keyup.enter="createFolder" />
              <button class="primary-btn" @click="createFolder" :disabled="folderBusy">{{ folderBusy ? 'Creating…' : 'Create' }}</button>
            </div>

            <div class="folder-panel-title" style="margin:18px 0 6px">Manage mail in a folder</div>
            <p class="muted" style="font-size:12px;margin-bottom:8px">Open any folder and copy, move or delete its mail — one at a time or many at once.</p>
            <div class="mailman-row">
              <select v-model="managerFolderId" class="folder-sel" style="flex:1" @change="managerLoad()">
                <option v-for="item in indentedFolders" :key="item.f.id" :value="item.f.id">{{ '　'.repeat(item.depth) }}{{ folderIcon(item.f) }} {{ item.f.name }}</option>
              </select>
              <button class="action-btn" @click="managerLoad()" :disabled="managerLoading">{{ managerLoading ? 'Loading…' : '↻ Refresh' }}</button>
            </div>

            <div v-if="managerShow" class="manager-panel">
              <div v-if="managerError" class="error-banner">{{ managerError }}</div>
              <div v-if="managerMsg" class="success-msg" style="margin:6px 0">{{ managerMsg }}</div>

              <div class="mailman-row manager-toolbar">
                <button class="pill" :class="{ active: managerAllChecked }" @click="managerSelectAll">
                  {{ managerSel.length ? managerSel.length + ' selected' : 'Select all' }}
                </button>
                <select v-model="managerDstFolderId" class="folder-sel" style="flex:1" title="Destination folder (for copy/move)">
                  <option value="">Destination folder…</option>
                  <option v-for="item in indentedFolders.filter(i => i.f.id !== managerFolderId)" :key="item.f.id" :value="item.f.id">{{ '　'.repeat(item.depth) }}{{ item.f.name }}</option>
                </select>
                <button class="action-btn" @click="managerCopyOrMove('copy')" :disabled="managerBusy" title="Copy selected mail to the destination">⧉ Copy</button>
                <button class="action-btn" @click="managerCopyOrMove('move')" :disabled="managerBusy" title="Move selected mail to the destination">➤ Move</button>
                <button class="action-btn danger-btn" @click="managerDelete" :disabled="managerBusy" title="Move selected mail to Trash">🗑 Delete</button>
              </div>

              <div v-if="managerLoading" class="empty-hint">Loading folder…</div>
              <div v-else-if="!managerEmails.length" class="empty-hint" style="padding:14px">This folder is empty.</div>
              <div v-else class="manager-list">
                <label v-for="e in managerEmails" :key="e.id" class="manager-mail">
                  <input type="checkbox" :checked="managerSel.includes(e.id + '\u0000' + e.serverId)" @change="managerToggle(e.id + '\u0000' + e.serverId)" />
                  <span class="manager-subj" :class="{ unread: !e.isRead }">{{ fmtMailmanSubject(e) }}</span>
                  <span class="manager-from">{{ e.fromEmail || e.from }}</span>
                  <span class="manager-date">{{ formatDate(e.dateReceived) }}</span>
                </label>
              </div>
            </div>
            <button v-if="!managerShow" class="action-btn muted" style="margin-top:4px" @click="managerOpen()">Open folder manager</button>
          </div>
        </div>
      </div>

      <!-- ==== SIGNATURES TAB ==== -->
      <div v-if="settingsTab === 'signatures'" class="settings-pane">
        <div class="settings-cols">
          <!-- left: editor -->
          <div class="settings-col">
            <div class="folder-panel-title" style="margin-bottom:6px">{{ sigEditIsNew ? 'New signature' : 'Edit signature' }}</div>
            <div class="form-group"><label>Name</label><input v-model="sigEditName" placeholder="e.g. Work, Personal, Freelance" /></div>
            <div class="form-group"><label>Signature text</label><textarea v-model="sigEditText" rows="4" placeholder="The signature text..." /></div>
            <div class="modal-actions" style="justify-content:flex-start">
              <button class="primary-btn" @click="saveSigEdit">Save Signature</button>
              <button class="action-btn muted" @click="startNewSig">New</button>
            </div>
          </div>
          <!-- right: saved list -->
          <div class="settings-col">
            <div class="folder-panel-title" style="margin-bottom:6px">Saved signatures</div>
            <div class="sig-list">
              <div v-for="s in signatures" :key="s.id" class="sig-item">
                <span class="sig-item-name">{{ s.name }}</span>
                <span class="sig-item-preview">{{ s.text.split('\n')[0] }}</span>
                <button class="icon-btn" @click="setDefault(s.id)" :title="s.id === sigDefaultId() ? 'Current default' : 'Set as default'">{{ s.id === sigDefaultId() ? '★' : '☆' }}</button>
                <button class="icon-btn" @click="startEditSig(s)" title="Edit">✏️</button>
                <button class="icon-btn" @click="deleteSig(s.id)" title="Delete">🗑</button>
              </div>
              <p v-if="signatures.length === 0" class="muted">No saved signatures yet — add one on the left.</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

</template>

<style>
@import './styles/main.css';

/* Clickable Frau Blücher logo → Easter-egg video */
.logo-link { display: inline-flex; align-items: center; cursor: pointer; text-decoration: none; line-height: 0; }
.logo-link:hover { opacity: 0.85; }

/* ===== UNIBOX: Chat-bubble email rows ===== */
.bubble-row {
  display: flex; align-items: flex-start; gap: 10px;
  width: 100%; padding: 12px 20px;
  background: none; border: none;
  border-bottom: 1px solid var(--border);
  color: var(--text); cursor: pointer; text-align: left;
  transition: background 0.15s;
}
.bubble-row:hover { background: var(--bg-hover); }

.bubble-sent {
  background: var(--accent-subtle);
}
.bubble-sent .bubble-content { border-left: 3px solid var(--accent); }
.bubble-sent:hover { background: rgba(91,127,255,0.12); }

.bubble-received .bubble-content { border-left: 3px solid transparent; }

.bubble-row.unread .bubble-subject { font-weight: 600; color: var(--text); }
.bubble-row.selected {
  background: var(--bg-selected) !important;
  box-shadow: inset 0 0 0 2px var(--accent);
}

/* Selection toolbar: shows when emails are multi-selected in a conversation */
.sel-toolbar {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-subtle);
  flex-shrink: 0;
}
.sel-count { font-size: 12px; font-weight: 600; color: var(--text); white-space: nowrap; }
.sel-target { max-width: 200px; }

/* Per-email hover actions in the bubble list */
.bubble-actions {
  display: flex; align-items: center; gap: 4px;
  flex-shrink: 0; align-self: center;
  opacity: 0; pointer-events: none;
  transition: opacity 0.12s;
}
.bubble-row:hover .bubble-actions { opacity: 1; pointer-events: auto; }
.bubble-act {
  position: relative;
  display: inline-flex; align-items: center;
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: 6px; padding: 4px 6px;
  font-size: 13px; cursor: pointer; color: var(--text-muted);
  transition: all 0.12s;
}
.bubble-act:hover { background: var(--bg-hover); color: var(--text); border-color: var(--accent); }
.bubble-act.danger-hover:hover { background: var(--danger-subtle, rgba(220,53,69,.12)); border-color: var(--danger, #dc3545); color: #dc3545; }
.bubble-move-sel {
  position: absolute; inset: 0; opacity: 0; cursor: pointer;
}
.bubble-act.selecting { z-index: 5; }

/* Message source viewer modal */
.source-modal { width: min(680px, 92vw); max-height: 86vh; display: flex; flex-direction: column; }
.source-scroll { overflow-y: auto; padding: 4px 20px 20px; }
.source-pre {
  background: #0f1115; color: #d7dae0; border: 1px solid var(--border);
  border-radius: 10px; padding: 12px 14px; font-size: 11.5px; line-height: 1.5;
  white-space: pre-wrap; word-break: break-word; max-height: 300px; overflow-y: auto;
}
.hdr-explain { display: flex; flex-direction: column; gap: 4px; }
.activity-modal { width: min(720px, 92vw); max-height: 82vh; display: flex; flex-direction: column; }
.activity-scroll { overflow-y: auto; padding: 4px 16px 8px; display: flex; flex-direction: column; gap: 4px; }
.activity-line {
  display: flex; gap: 8px; align-items: baseline; padding: 4px 6px; border-radius: 6px;
  font-size: 12px; line-height: 1.45; white-space: pre-wrap; word-break: break-word;
}
.activity-line:nth-child(odd) { background: rgba(127,127,127,0.06); }
.activity-kind {
  flex: 0 0 64px; font-family: ui-monospace, monospace; font-weight: 700; font-size: 10.5px;
  letter-spacing: 0.04em; text-transform: uppercase;
}
.activity-text { font-family: ui-monospace, monospace; font-size: 11.5px; }
.k-recv { color: #2e9e5b; }
.k-send { color: #3b82f6; }
.k-other { color: #8b8f98; }
.activity-meta { font-size: 12.5px; padding: 0 4px; }
.hdr-row {
  display: flex; gap: 12px; align-items: baseline; padding: 5px 8px;
  border-radius: 8px; background: var(--bg-card-subtle, rgba(255,255,255,.02));
}
.hdr-row:hover { background: var(--bg-hover); }
.hdr-name { flex-shrink: 0; min-width: 150px; font-weight: 600; color: var(--accent); }
.hdr-desc { color: var(--text-muted); font-size: 13px; }

/* Move-to-folder picker + new folder */
.mailman-panel {
  border: 1px solid var(--border-light); border-radius: 10px;
  padding: 12px 14px; margin-bottom: 18px; background: var(--accent-subtle);
}
.mailman-row {
  display: flex; align-items: center; gap: 10px; margin-top: 8px; flex-wrap: wrap;
}
.mailman-row .primary-btn, .mailman-row .action-btn { white-space: nowrap; }

.bubble-avatar-col { flex-shrink: 0; width: 32px; display: flex; align-items: flex-start; padding-top: 2px; }
.avatar-xs { width: 28px; height: 28px; border-radius: 50%; object-fit: cover; border: 1px solid var(--border); }
.bubble-you {
  font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;
  color: var(--accent); background: var(--accent-subtle); border: 1px solid var(--accent);
  border-radius: 10px; padding: 2px 8px; white-space: nowrap;
}

.bubble-content {
  flex: 1; min-width: 0; padding-left: 4px;
}
.bubble-top-row {
  display: flex; align-items: center; gap: 8px; margin-bottom: 2px;
}
.bubble-from {
  font-size: 13px; font-weight: 500; color: var(--text);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.bubble-folder {
  font-size: 10px; color: var(--text-dim); background: var(--bg-card);
  padding: 1px 6px; border-radius: 8px; white-space: nowrap;
}
.bubble-date {
  font-size: 11px; color: var(--text-dim); margin-left: auto; white-space: nowrap;
}
.bubble-subject {
  font-size: 14px; font-weight: 450; color: var(--text-muted);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  margin-bottom: 2px;
}
.bubble-preview {
  font-size: 12px; color: var(--text-dim);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; line-height: 1.4;
}
.bubble-attach {
  font-size: 12px; margin-top: 2px;
}

/* Contacts + Diary views */
.contacts-modal { width: 480px; max-width: 92vw; }
.contact-list { max-height: 62vh; overflow-y: auto; display: flex; flex-direction: column; }
.contact-row {
  display: flex; align-items: center; gap: 10px; width: 100%;
  padding: 8px 12px; background: none; border: none; border-bottom: 1px solid var(--border);
  color: var(--text); cursor: pointer; text-align: left;
}
.contact-row:hover { background: rgba(91,127,255,0.10); }
.avatar-sm { width: 36px; height: 36px; border-radius: 50%; object-fit: cover; border: 1px solid var(--border); }
.contact-meta { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.contact-name { font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.contact-email { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.contact-actions { color: var(--text-dim); }

/* ===== DIARY — day / week / month calendar ===== */
.diary-modal { width: min(920px, 96vw); height: min(84vh, 760px); display: flex; flex-direction: column; }
.diary-modal .modal-header { flex: 0 0 auto; }
.diary-modal .modal-body { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; }

/* Fixed-height, single-row toolbar: a stable grid so the day/date never
   moves up or down as you switch views or navigate (the old flex-wrap let a
   long Day title wrap onto a second line, shifting everything each change). */
.diary-toolbar {
  display: grid; grid-template-columns: auto auto 1fr; align-items: center; gap: 12px;
  padding: 8px 16px 12px; border-bottom: 1px solid var(--border); flex: 0 0 auto;
  min-height: 44px;
}
.diary-viewswitch { display: flex; gap: 4px; background: var(--bg-card); border: 1px solid var(--border); border-radius: 10px; padding: 3px; min-width: 0; }
.view-btn {
  font-size: 12px; font-weight: 600; color: var(--text-muted); background: transparent;
  border: none; border-radius: 7px; padding: 4px 12px; cursor: pointer; transition: all 0.15s;
  white-space: nowrap;
}
.view-btn:hover { color: var(--text); }
.view-btn.active { background: var(--accent); color: #fff; }
.diary-nav { display: flex; align-items: center; gap: 6px; justify-content: center; }
.diary-nav .icon-btn { font-size: 16px; }
.diary-title {
  font-weight: 700; font-size: 16px; color: var(--text); text-align: right;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0;
}

.diary-modal .diary-dayview, .diary-modal .diary-monthview { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; }

.diary-list { overflow-y: auto; display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.diary-empty { flex-direction: column; gap: 6px; align-items: center; padding: 50px 0; justify-content: center; }

/* Shared event card */
.diary-card {
  display: flex; align-items: center; gap: 12px;
  padding: 11px 12px; border-radius: var(--radius-sm);
  border: 1px solid var(--border); background: var(--bg-card);
  transition: border-color 0.15s, background 0.15s;
}
.diary-card:hover { border-color: var(--accent); background: var(--bg-hover); }
.diary-card-time {
  display: flex; flex-direction: column; align-items: flex-end;
  min-width: 78px; max-width: 78px; gap: 3px; flex-shrink: 0;
}
.diary-time-main {
  font-weight: 600; font-size: 12.5px; color: var(--accent-hover);
  font-variant-numeric: tabular-nums; white-space: nowrap;
}
.diary-card-time.allday .diary-time-main { color: var(--success); font-size: 11.5px; text-transform: uppercase; letter-spacing: 0.3px; }
.diary-sync { font-size: 9px; color: var(--text-dim); text-transform: uppercase; letter-spacing: 0.4px; }
.diary-card-line { width: 2px; align-self: stretch; border-radius: 2px; background: var(--accent-glow); }
.diary-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.diary-subject { font-weight: 600; font-size: 13.5px; color: var(--text); }
.diary-meta { display: flex; flex-wrap: wrap; gap: 6px; }
.diary-chip {
  font-size: 11px; color: var(--text-muted); background: var(--bg-surface);
  border: 1px solid var(--border-light); padding: 2px 8px; border-radius: 20px;
  white-space: nowrap; max-width: 100%; overflow: hidden; text-overflow: ellipsis;
}
.diary-actions {
  display: flex; gap: 4px; align-items: center; flex-shrink: 0;
  opacity: 0; pointer-events: none; transition: opacity 0.15s;
}
.diary-card:hover .diary-actions { opacity: 1; pointer-events: auto; }
.diary-actions .icon-btn { font-size: 13px; background: var(--bg-surface); border: 1px solid var(--border-light); }
.diary-actions .icon-btn:hover { border-color: var(--accent); background: var(--bg-hover); }
.diary-actions .icon-btn.danger { color: #ff6b6b; }
.diary-actions .icon-btn.danger:hover { border-color: var(--danger); background: rgba(255,92,92,0.12); }

/* --- Week view --- */
.diary-week { flex: 1 1 auto; min-height: 0; display: grid; grid-template-columns: repeat(7, 1fr); gap: 8px; padding: 14px 16px; }
.diary-weekday {
  display: flex; flex-direction: column; min-height: 0; border: 1px solid var(--border);
  border-radius: var(--radius-sm); background: var(--bg-card); overflow: hidden;
}
.diary-weekday.today { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.diary-weekday-head {
  display: flex; flex-direction: column; align-items: center; gap: 2px;
  padding: 8px 4px 6px; border-bottom: 1px solid var(--border); background: var(--bg-surface);
}
.diary-weekday-name { font-size: 10px; text-transform: uppercase; letter-spacing: 0.6px; color: var(--text-dim); }
.pill-today,
.diary-weekday-num {
  font-size: 15px; font-weight: 700; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center;
  border-radius: 50%; color: var(--text);
}
.diary-weekday-num.pill-today { background: var(--accent); color: #fff; }
.diary-weekday-body { flex: 1; min-height: 0; overflow-y: auto; padding: 6px; display: flex; flex-direction: column; gap: 4px; }
.diary-wev {
  font-size: 11px; border-left: 3px solid var(--accent); background: var(--accent-subtle);
  border-radius: 4px; padding: 3px 5px; cursor: default; line-height: 1.3;
}
.diary-wev:hover { background: var(--accent-glow); }
.diary-wtime { font-weight: 700; color: var(--accent-hover); font-variant-numeric: tabular-nums; }
.diary-wsubj { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.diary-weekday-empty { color: transparent; user-select: none; }

/* --- Month view --- */
.diary-grid-head { display: grid; grid-template-columns: repeat(7, 1fr); gap: 6px; padding: 10px 16px 4px; }
.diary-grid-wd { font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.6px; color: var(--text-dim); text-align: center; }
.diary-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 6px; padding: 4px 16px 8px; flex: 1 1 auto; min-height: 0; }
.diary-grid-cell {
  border: 1px solid var(--border); background: var(--bg-card); border-radius: var(--radius-sm);
  display: flex; flex-direction: column; padding: 5px; min-height: 62px; cursor: pointer;
  text-align: left; color: var(--text); align-items: stretch; transition: border-color 0.15s, background 0.15s;
}
.diary-grid-cell:hover { border-color: var(--accent); }
.diary-grid-cell.out { opacity: 0.38; background: transparent; }
.diary-grid-cell.today { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.diary-grid-cell.sel { background: var(--accent-subtle); border-color: var(--accent-hover); }
.diary-cell-day { font-size: 12px; font-weight: 600; }
.diary-grid-cell.today .diary-cell-day { color: var(--accent-hover); }
.diary-cell-evs { display: flex; flex-direction: column; gap: 2px; margin-top: 3px; overflow: hidden; }
.diary-cell-ev {
  font-size: 9.5px; background: var(--accent-subtle); border-left: 2px solid var(--accent);
  padding: 1px 3px; border-radius: 3px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.diary-cell-more { font-size: 9px; color: var(--text-dim); }
.diary-selday { flex: 0 0 auto; max-height: 30%; display: flex; flex-direction: column; border-top: 1px solid var(--border); }
.diary-selday-head { padding: 8px 16px 2px; }
.diary-selday-label { font-weight: 700; font-size: 13.5px; }
.diary-selday .diary-list { gap: 8px; padding: 8px 16px 14px; }

.danger-btn { background: #c0392b !important; }

/* Settings → Mail & Folders → per-folder mail manager */
.manager-panel {
  margin-top: 10px; padding: 12px; border: 1px solid var(--border-light);
  border-radius: var(--radius-sm); background: var(--bg-card);
}
.manager-toolbar { margin-bottom: 10px; }
.manager-list {
  display: flex; flex-direction: column; gap: 6px; max-height: 320px;
  overflow-y: auto; padding-right: 2px;
}
.manager-mail {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 10px; border-radius: var(--radius-sm);
  border: 1px solid var(--border); background: var(--bg-surface);
  transition: border-color 0.15s, background 0.15s; cursor: pointer;
}
.manager-mail:hover { border-color: var(--accent); background: var(--bg-hover); }
.manager-mail input[type=checkbox] { accent-color: var(--accent); flex-shrink: 0; }
.manager-subj { flex: 1; font-size: 13px; font-weight: 400; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.manager-subj.unread { font-weight: 600; }
.manager-from { font-size: 12px; color: var(--text-muted); min-width: 120px; max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.manager-date { font-size: 11.5px; color: var(--text-dim); white-space: nowrap; font-variant-numeric: tabular-nums; }

/* Compose To autocomplete dropdown */
/* New Event form helpers */
.modal-header-actions { display: flex; align-items: center; gap: 8px; }
.modal-body { padding: 4px 2px; display: flex; flex-direction: column; gap: 12px; }
.row-fields { display: flex; gap: 10px; }
.row-fields > div { flex: 1; }
.check-label { display: flex; align-items: center; gap: 8px; font-weight: 500; }
.dt-fields { display: flex; gap: 6px; }
.dt-date { flex: 3; min-width: 0; }
.dt-time { flex: 2; min-width: 0; }
.dt-fields input { font-variant-numeric: tabular-nums; }

.compose-suggest {
  position: absolute; margin-top: 2px; max-height: 240px; overflow-y: auto;
  width: 100%; background: var(--bg-card, #fff); border: 1px solid var(--border);
  border-radius: 8px; box-shadow: 0 4px 16px rgba(0,0,0,0.12); z-index: 50;
}
.suggest-item {
  display: flex; flex-direction: column; width: 100%; text-align: left;
  padding: 6px 10px; background: none; border: none; border-bottom: 1px solid var(--border);
  cursor: pointer; color: var(--text);
}
.suggest-item:last-child { border-bottom: none; }
.suggest-item:hover { background: rgba(91,127,255,0.12); }
.suggest-addr { font-weight: 500; font-size: 13px; }
.suggest-name { font-size: 11px; color: var(--text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>