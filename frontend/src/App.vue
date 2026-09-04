<template>
  <div class="app" :class="{ 'has-account': currentAccount }">
    <!-- Sidebar: Accounts + Search -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1 class="logo">✉️ EasyMail</h1>
        <button class="btn-icon" @click="showAddAccount = true" title="Add Account">+</button>
      </div>

      <!-- Account selector -->
      <div class="account-list" v-if="accounts.length > 0">
        <div
          v-for="acc in accounts"
          :key="acc.id"
          class="account-item"
          :class="{ active: currentAccount?.id === acc.id, connected: acc.connected }"
          @click="selectAccount(acc)"
        >
          <span class="account-dot" :class="acc.connected ? 'online' : 'offline'"></span>
          <span class="account-name">{{ acc.name || acc.email }}</span>
        </div>
      </div>

      <div class="empty-state" v-if="accounts.length === 0">
        <p>No accounts yet</p>
        <button class="btn-primary" @click="showAddAccount = true">Add Account</button>
      </div>

      <!-- Search -->
      <div class="search-box" v-if="currentAccount">
        <input
          type="text"
          placeholder="Search contacts..."
          v-model="searchQuery"
          @input="searchContacts"
        />
      </div>

      <!-- Contact list (UniBox-style: people sorted by last email) -->
      <div class="contact-list" v-if="currentAccount">
        <div
          v-for="contact in filteredContacts"
          :key="contact.id"
          class="contact-item"
          :class="{ active: selectedContact?.id === contact.id }"
          @click="selectContact(contact)"
        >
          <div class="contact-avatar" :style="{ background: avatarColor(contact.email) }">
            {{ contactInitials(contact) }}
          </div>
          <div class="contact-info">
            <div class="contact-name">{{ contact.name || contact.email }}</div>
            <div class="contact-preview">{{ contact.email }}</div>
          </div>
          <div class="contact-meta">
            <span class="contact-count">{{ contact.emailCount }}</span>
            <span class="contact-time">{{ relativeTime(contact.lastEmailAt) }}</span>
          </div>
        </div>
      </div>
    </aside>

    <!-- Middle: Email conversation -->
    <main class="conversation" v-if="selectedContact">
      <div class="conversation-header">
        <h2>{{ selectedContact.name || selectedContact.email }}</h2>
        <span class="contact-email-header">{{ selectedContact.email }}</span>
      </div>

      <div class="email-list">
        <div
          v-for="email in emails"
          :key="email.id"
          class="email-item"
          :class="{ unread: !email.isRead, selected: selectedEmail?.id === email.id }"
          @click="selectEmail(email)"
        >
          <div class="email-header">
            <span class="email-from">{{ email.from || email.fromEmail }}</span>
            <span class="email-time">{{ formatTime(email.dateReceived) }}</span>
          </div>
          <div class="email-subject">{{ email.subject }}</div>
          <div class="email-preview">{{ email.preview }}</div>
          <div class="email-badges">
            <span v-if="email.hasAttachment" class="badge">📎</span>
            <span v-if="email.importance === 2" class="badge important">!</span>
            <span v-if="email.isFlagged" class="badge">⭐</span>
          </div>
        </div>
      </div>
    </main>

    <!-- Right: Email detail / compose -->
    <section class="detail" v-if="selectedEmail">
      <div class="detail-header">
        <h3>{{ selectedEmail.subject }}</h3>
        <div class="detail-actions">
          <button class="btn-icon" title="Reply">↩️</button>
          <button class="btn-icon" title="Forward">➡️</button>
          <button class="btn-icon" title="Archive">📦</button>
          <button class="btn-icon" title="Delete">🗑️</button>
        </div>
      </div>
      <div class="detail-meta">
        <strong>{{ selectedEmail.from || selectedEmail.fromEmail }}</strong>
        <span>to {{ selectedEmail.to }}</span>
        <span class="detail-date">{{ formatDate(selectedEmail.dateReceived) }}</span>
      </div>
      <div class="detail-body" v-html="selectedEmail.body || selectedEmail.preview"></div>
      <div class="detail-attachments" v-if="selectedEmail.hasAttachment">
        <h4>Attachments</h4>
        <div class="attachment-grid">
          <!-- Attachment grid view, UniBox-style -->
        </div>
      </div>
    </section>

    <!-- Empty states -->
    <div class="empty-detail" v-if="!selectedContact && currentAccount">
      <div class="empty-icon">👥</div>
      <p>Select a contact to view emails</p>
    </div>
    <div class="empty-detail" v-if="!currentAccount">
      <div class="empty-icon">✉️</div>
      <p>Add an account to get started</p>
    </div>

    <!-- Add Account Modal -->
    <div class="modal-overlay" v-if="showAddAccount" @click.self="showAddAccount = false">
      <div class="modal">
        <h2>Add Account</h2>
        <div class="form-group">
          <label>Account Name</label>
          <input v-model="newAccount.name" placeholder="e.g. Work" />
        </div>
        <div class="form-group">
          <label>Email Address</label>
          <input v-model="newAccount.email" type="email" placeholder="rupert@btl.io" />
        </div>
        <div class="form-group">
          <label>Password</label>
          <input v-model="newAccount.password" type="password" />
        </div>
        <div class="form-group">
          <label>Server URL</label>
          <input v-model="newAccount.serverUrl" placeholder="https://mex.btl.io/Microsoft-Server-ActiveSync" />
          <small>Leave blank for Autodiscover</small>
        </div>
        <div class="form-actions">
          <button class="btn-secondary" @click="showAddAccount = false">Cancel</button>
          <button class="btn-primary" @click="addAccount">Connect</button>
        </div>
        <div class="form-error" v-if="accountError">{{ accountError }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  AddAccount,
  ListAccounts,
  RemoveAccount,
  GetContacts,
  SearchContacts,
  GetEmailsByContact,
  GetFolders,
  SyncFolders,
} from '../wailsjs/go/app/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { models } from '../wailsjs/go/models'

interface Account {
  id: string
  name: string
  email: string
  connected: boolean
}

interface Contact {
  id: string
  name: string
  email: string
  emailCount: number
  lastEmailAt: string
  isFavorite: boolean
}

interface Email {
  id: string
  from: string
  fromEmail: string
  to: string
  subject: string
  dateReceived: string
  isRead: boolean
  isFlagged: boolean
  importance: number
  hasAttachment: boolean
  preview: string
  body: string
}

const accounts = ref<Account[]>([])
const currentAccount = ref<Account | null>(null)
const contacts = ref<Contact[]>([])
const selectedContact = ref<Contact | null>(null)
const emails = ref<Email[]>([])
const selectedEmail = ref<Email | null>(null)
const searchQuery = ref('')
const showAddAccount = ref(false)
const accountError = ref('')

const newAccount = ref({
  name: '',
  email: '',
  password: '',
  serverUrl: '',
})

const filteredContacts = computed(() => {
  if (!searchQuery.value) return contacts.value
  const q = searchQuery.value.toLowerCase()
  return contacts.value.filter(
    (c) =>
      c.name?.toLowerCase().includes(q) || c.email.toLowerCase().includes(q)
  )
})

async function loadAccounts() {
  try {
    accounts.value = await ListAccounts()
    if (accounts.value.length > 0 && !currentAccount.value) {
      selectAccount(accounts.value[0])
    }
  } catch (e) {
    console.error('Failed to load accounts:', e)
  }
}

async function selectAccount(acc: Account) {
  currentAccount.value = acc
  contacts.value = await GetContacts(acc.id)
  selectedContact.value = null
  emails.value = []
  selectedEmail.value = null
  // Sync folders in background
  try {
    await SyncFolders(acc.id)
  } catch (e) {
    console.error('Folder sync failed:', e)
  }
}

async function selectContact(contact: Contact) {
  selectedContact.value = contact
  selectedEmail.value = null
  if (currentAccount.value) {
    try {
      emails.value = await GetEmailsByContact(
        currentAccount.value.id,
        contact.email,
        0,
        50
      )
    } catch (e) {
      console.error('Failed to load emails:', e)
      emails.value = []
    }
  }
}

function selectEmail(email: Email) {
  selectedEmail.value = email
}

async function addAccount() {
  accountError.value = ''
  try {
    const acc = await AddAccount(
      newAccount.value.name,
      newAccount.value.email,
      newAccount.value.password,
      newAccount.value.serverUrl
    )
    showAddAccount.value = false
    newAccount.value = { name: '', email: '', password: '', serverUrl: '' }
    await loadAccounts()
    if (acc) selectAccount(acc)
  } catch (e: any) {
    accountError.value = e.message || String(e)
  }
}

async function searchContacts() {
  if (!currentAccount.value) return
  if (searchQuery.value.length > 1) {
    try {
      contacts.value = await SearchContacts(
        currentAccount.value.id,
        searchQuery.value
      )
    } catch (e) {
      console.error('Search failed:', e)
    }
  } else {
    contacts.value = await GetContacts(currentAccount.value.id)
  }
}

// Color generation for avatars
function avatarColor(email: string): string {
  const colors = [
    '#e57373', '#f06292', '#ba68c8', '#9575cd',
    '#7986cb', '#64b5f6', '#4fc3f7', '#4dd0e1',
    '#4db6ac', '#81c784', '#aed581', '#dce775',
    '#fff176', '#ffd54f', '#ffb74d', '#ff8a65',
  ]
  let hash = 0
  for (let i = 0; i < email.length; i++) {
    hash = email.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}

function contactInitials(contact: Contact): string {
  if (contact.name) {
    const parts = contact.name.split(' ')
    return parts.map((p) => p[0]).join('').toUpperCase().slice(0, 2)
  }
  return contact.email[0].toUpperCase()
}

function relativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'now'
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d`
  return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
}

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleTimeString('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-GB', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

onMounted(() => {
  loadAccounts()
})
</script>