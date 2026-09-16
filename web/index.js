(function () {
  var form = document.getElementById('dev-form')
  var input = document.getElementById('dev-user')
  var msg = document.getElementById('dev-msg')
  var loginLink = document.getElementById('login-link')
  var contestLink = document.getElementById('contest-link')
  var loginStatus = document.getElementById('login-status')

  function applyLoginState (authenticated, username) {
    if (authenticated) {
      loginLink.hidden = true
      contestLink.hidden = false
      loginStatus.textContent = username ? ('已登录：' + username) : '已登录'
    } else {
      loginLink.hidden = false
      contestLink.hidden = true
      loginStatus.textContent = '未登录，请先通过 CAS 登录'
    }
  }

  fetch('/api/me', { credentials: 'same-origin' })
    .then(function (r) { return r.json().then(function (body) { return { ok: r.ok, body: body } }) })
    .then(function (res) {
      if (!res.ok) {
        applyLoginState(false)
        return
      }
      applyLoginState(true, res.body.username)
    })
    .catch(function () {
      applyLoginState(false)
      loginStatus.textContent = '暂时无法检查登录状态'
    })

  form.addEventListener('submit', function (e) {
    e.preventDefault()
    var user = input.value.trim()
    if (!user) return
    fetch('/auth/dev?username=' + encodeURIComponent(user), { credentials: 'same-origin' })
      .then(function (r) { return r.json().then(function (body) { return { ok: r.ok, body: body } }) })
      .then(function (res) {
        if (!res.ok) throw new Error(res.body.error || '开发登录失败')
        msg.textContent = '已登录：' + res.body.username
        window.location.href = '/contest'
      })
      .catch(function (err) { msg.textContent = err.message; msg.className = 'error' })
  })
}())
