// 所有页面共享的工具：API 请求、CSRF、登出、移动端菜单。
// 该文件是 ES module，页面脚本通过 import 复用，模块只执行一次。

// csrfToken 从双提交 Cookie 中取出 CSRF Token。
export function csrfToken () {
  const m = document.cookie.match(/(?:^|; )contest_csrf=([^;]*)/)
  return m ? decodeURIComponent(m[1]) : ''
}

// api 封装同源 JSON 请求：自动携带 CSRF Token、解析错误体。
// 401 表示会话失效，统一跳回首页。
// 成功时返回解析后的 JSON（204 返回 null），失败抛出带 .status 的 Error。
export async function api (path, { method = 'GET', body } = {}) {
  const headers = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET') headers['X-CSRF-Token'] = csrfToken()

  const resp = await fetch(path, {
    method,
    headers,
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body)
  })

  if (resp.status === 401) {
    window.location.href = '/'
    throw new Error('登录已失效')
  }
  if (resp.status === 204) return null

  const data = await resp.json().catch(() => null)
  if (!resp.ok) {
    const err = new Error((data && data.error) || ('HTTP ' + resp.status))
    err.status = resp.status
    throw err
  }
  return data
}

// logout 调用登出接口；若后端返回 CAS 登出地址则跳转过去完成 SSO 登出。
async function logout () {
  let target = '/'
  try {
    const data = await api('/auth/logout', { method: 'POST' })
    if (data && data.logout_url) target = data.logout_url
  } catch {
    // 网络失败也回到首页，本地 Cookie 已无法通过接口清除时由过期兜底
  }
  window.location.href = target
}

// 绑定全站 UI：登出按钮与移动端菜单。
// 模块脚本在 DOM 解析完成后执行，无需等待 DOMContentLoaded。
function bindGlobalUI () {
  document.querySelectorAll('[data-action="logout"]').forEach(function (el) {
    el.addEventListener('click', function (e) {
      e.preventDefault()
      logout()
    })
  })

  const toggle = document.querySelector('[data-action="toggle-mobile-menu"]')
  const menu = document.getElementById('mobile-menu')
  if (!toggle || !menu) return

  const openIcon = toggle.querySelector('[data-menu-icon="open"]')
  const closeIcon = toggle.querySelector('[data-menu-icon="close"]')
  toggle.addEventListener('click', function () {
    const opening = menu.classList.contains('hidden')
    menu.classList.toggle('hidden', !opening)
    if (openIcon) openIcon.classList.toggle('hidden', opening)
    if (closeIcon) closeIcon.classList.toggle('hidden', !opening)
    toggle.setAttribute('aria-expanded', opening ? 'true' : 'false')
  })
}

bindGlobalUI()
