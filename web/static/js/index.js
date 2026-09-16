// 主页：按剩余次数切换“前往答题”按钮状态，处理开发登录。
import { api } from './common.js'

const cta = document.getElementById('url-quiz-contest')
const ctaText = document.getElementById('url-quiz-contest-text')
const noTries = document.getElementById('no-tries')

function showGuestCta () {
  cta.href = '/auth/cas/login'
  cta.classList.remove('hidden')
  ctaText.textContent = '登录并前往答题'
}

function showAuthedCta () {
  cta.href = '/contest'
  cta.classList.remove('hidden')
  ctaText.textContent = '前往答题'
  noTries.classList.add('hidden')
}

async function refreshCta () {
  if (document.body.dataset.authenticated !== 'true') {
    showGuestCta()
    return
  }
  try {
    const body = await api('/api/scores')
    if (body.attempts_left > 0) {
      showAuthedCta()
    } else {
      cta.classList.add('hidden')
      noTries.classList.remove('hidden')
    }
  } catch (err) {
    if (err.message === '登录已失效') return
    // 读取失败时兜底为可点击状态，由答题页接口再次校验
    showAuthedCta()
  }
}

function bindDevLogin () {
  const form = document.getElementById('dev-form')
  if (!form) return
  const input = document.getElementById('dev-user')
  const msg = document.getElementById('dev-msg')

  form.addEventListener('submit', async function (e) {
    e.preventDefault()
    const username = input.value.trim()
    if (!username) return
    try {
      await api('/auth/dev?username=' + encodeURIComponent(username))
      window.location.href = '/contest'
    } catch (err) {
      msg.textContent = err.message
      msg.className = 'mt-2 text-sm text-red-700'
    }
  })
}

refreshCta()
bindDevLogin()
