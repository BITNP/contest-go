// 答题页：加载试卷、自动保存答案、倒计时、手动/自动交卷。
import { api, csrfToken } from './common.js'

const SAVE_DEBOUNCE_MS = 350

const form = document.getElementById('exam-form')
const errorBox = document.getElementById('error')
const statusBox = document.getElementById('save-status')
const submitBtn = document.getElementById('submit-btn')
const modal = document.getElementById('modal')
const contestText = document.getElementById('contest-progress-text')
const contestBar = document.getElementById('contest-progress-bar')
const timeText = document.getElementById('time-progress-text')
const timeBar = document.getElementById('time-progress-bar')

// exam 为 /api/exam 返回的试卷；clockOffsetMS 用于把本机时间校准到服务器时间。
let exam = null
let clockOffsetMS = 0
// pending: qid -> { question_id, choice_ids }，等待防抖发送或失败后待重发的答案。
const pending = new Map()
const timers = new Map()
let inFlight = 0
let saveFailed = false
let submitting = false
let autoSubmitted = false

function serverNow () {
  return Date.now() + clockOffsetMS
}

function showError (msg, { retry } = {}) {
  errorBox.textContent = String(msg)
  if (retry) {
    const btn = document.createElement('button')
    btn.type = 'button'
    btn.className = 'error-retry'
    btn.textContent = '重试'
    btn.addEventListener('click', function () {
      hideError()
      retry()
    })
    errorBox.appendChild(btn)
  }
  errorBox.classList.remove('hidden')
}

function hideError () {
  errorBox.classList.add('hidden')
  errorBox.textContent = ''
}

function updateSaveStatus () {
  if (submitting || !exam) return
  if (inFlight > 0) {
    statusBox.textContent = '正在保存……'
  } else if (saveFailed) {
    statusBox.textContent = '部分答案尚未保存，继续作答或提交时会自动重试'
  } else {
    statusBox.textContent = '全部答案已保存 ' + new Date().toLocaleTimeString()
  }
}

// sendAnswer 发送一题答案。失败时把答案放回 pending，等待下次修改或交卷时重发。
async function sendAnswer (qid) {
  const body = pending.get(qid)
  if (!body) return true
  pending.delete(qid)
  clearTimeout(timers.get(qid))
  timers.delete(qid)

  inFlight++
  updateSaveStatus()
  try {
    await api('/api/answer', { method: 'POST', body })
    saveFailed = pending.size > 0 // 其余题仍处于失败重发队列时保持提示
    return true
  } catch (err) {
    if (!pending.has(qid)) pending.set(qid, body)
    saveFailed = true
    showError('保存失败：' + err.message)
    return false
  } finally {
    inFlight--
    updateSaveStatus()
  }
}

function scheduleAnswer (qid, choiceIDs) {
  pending.set(qid, { question_id: qid, choice_ids: choiceIDs })
  clearTimeout(timers.get(qid))
  timers.set(qid, setTimeout(function () { sendAnswer(qid) }, SAVE_DEBOUNCE_MS))
  updateSaveStatus()
}

function collectChoices (qid) {
  const ids = []
  form.querySelectorAll('input[data-qid="' + qid + '"]:checked').forEach(function (el) {
    ids.push(Number(el.value))
  })
  return ids
}

function scorePerQuestion () {
  if (!exam || !exam.questions.length) return 5
  return Math.round(exam.total_score / exam.questions.length)
}

// renderQuestion 生成一题的 fieldset。
function renderQuestion (q, index, selected) {
  const fs = document.createElement('fieldset')
  fs.className = 'my-4 p-4 sm:px-6 lg:px-8 bg-white shadow'

  // fieldset 的直接子 legend 会被浏览器强制特殊定位（骑在边框线上），
  // 这里用普通 <p> 作题目标题；fieldset 不带 legend 也是合法 HTML。
  const heading = document.createElement('p')
  heading.className = 'text-lg my-0'
  const num = document.createElement('span')
  num.className = 'text-gray-600'
  num.textContent = (index + 1) + '. '
  const title = document.createElement('span')
  title.className = 'font-bold'
  title.textContent = q.content
  const score = document.createElement('span')
  score.className = 'text-gray-600'
  score.textContent = '（' + scorePerQuestion() + '分）'
  heading.appendChild(num)
  heading.appendChild(title)
  heading.appendChild(score)
  fs.appendChild(heading)

  for (const c of q.choices) {
    const label = document.createElement('label')
    label.className = 'block hover:bg-red-100 my-2'
    const input = document.createElement('input')
    input.type = q.category === 'M' ? 'checkbox' : 'radio'
    input.name = 'q-' + q.id
    input.value = String(c.id)
    input.dataset.qid = String(q.id)
    input.checked = selected.includes(c.id)
    label.appendChild(input)
    label.appendChild(document.createTextNode(' ' + c.content))
    fs.appendChild(label)
  }
  return fs
}

function render () {
  form.textContent = ''
  exam.questions.forEach(function (q, index) {
    const selected = (exam.answers && exam.answers[q.id]) || []
    form.appendChild(renderQuestion(q, index, selected))
  })
  updateProgress()
}

function updateProgress () {
  if (!exam) return
  let answered = 0
  exam.questions.forEach(function (q) {
    if (collectChoices(q.id).length > 0) answered++
  })
  contestText.textContent = String(exam.questions.length - answered)
  contestBar.style.width = exam.questions.length
    ? (100 * answered / exam.questions.length) + '%'
    : '0%'
}

function formatLeft (leftMS) {
  const totalSec = Math.max(0, Math.ceil(leftMS / 1000))
  if (totalSec < 60) return '约' + totalSec + '秒'
  const min = Math.floor(totalSec / 60)
  const sec = totalSec % 60
  return min + '分' + String(sec).padStart(2, '0') + '秒'
}

function updateTimer () {
  if (!exam) return
  const left = Math.max(0, exam.deadline_unix_ms - serverNow())
  const progress = Math.min(1, Math.max(0, 1 - left / 1000 / exam.deadline_seconds))
  timeBar.style.width = (100 * progress) + '%'
  timeText.textContent = formatLeft(left)
  if (progress > 0.85) {
    timeBar.classList.add('time-progress-severe')
  }
  if (left <= 0 && !autoSubmitted) {
    autoSubmitted = true
    doSubmit(true)
  }
}

async function doSubmit (auto) {
  if (submitting) return
  submitting = true
  submitBtn.disabled = true
  statusBox.textContent = auto ? '时间到，正在自动提交……' : '正在提交……'
  try {
    const results = await Promise.all([...pending.keys()].map(sendAnswer))
    if (results.some(function (ok) { return !ok })) {
      throw new Error('部分答案保存失败，请检查网络后重试')
    }
    await api('/api/submit', { method: 'POST' })
    window.location.href = '/info'
  } catch (err) {
    submitting = false
    submitBtn.disabled = false
    statusBox.textContent = ''
    showError('提交失败：' + err.message, {
      retry: function () { doSubmit(auto) }
    })
  }
}

async function loadExam () {
  try {
    const body = await api('/api/exam')
    exam = body
    // 用服务器当前时间校准本机时钟，避免客户端时间不准导致倒计时偏差。
    clockOffsetMS = body.now_unix_ms - Date.now()
    render()
    updateTimer()
    setInterval(updateTimer, 200)
    hideError()
    submitBtn.disabled = false
  } catch (err) {
    if (err.message === '登录已失效') return
    showError('加载试卷失败：' + err.message, { retry: loadExam })
  }
}

// 事件委托：一个监听器覆盖所有选项，无需逐题绑定。
form.addEventListener('change', function (e) {
  const input = e.target.closest('input[data-qid]')
  if (!input) return
  const qid = Number(input.dataset.qid)
  scheduleAnswer(qid, collectChoices(qid))
  updateProgress()
})

// 试卷加载完成前禁止提交（loadExam 成功后重新启用）。
submitBtn.disabled = true

submitBtn.addEventListener('click', function () {
  if (submitting) return
  modal.classList.add('show')
})
document.getElementById('cancel-submit').addEventListener('click', function () {
  modal.classList.remove('show')
})
document.getElementById('confirm-submit').addEventListener('click', function () {
  modal.classList.remove('show')
  doSubmit(false)
})

// 页面关闭/刷新前尽力把未保存的答案发出去。
window.addEventListener('pagehide', function () {
  pending.forEach(function (body) {
    fetch('/api/answer', {
      method: 'POST',
      credentials: 'same-origin',
      keepalive: true,
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken() },
      body: JSON.stringify(body)
    })
  })
})

loadExam()
