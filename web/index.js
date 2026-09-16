(function () {
  'use strict'

  if (document.getElementById('score-rows')) return

  var form = document.getElementById('dev-form')
  var input = document.getElementById('dev-user')
  var msg = document.getElementById('dev-msg')
  var cta = document.getElementById('url-quiz-contest')
  var ctaText = document.getElementById('url-quiz-contest-text')
  var noTries = document.getElementById('no-tries')

  function showGuestCta () {
    if (!cta) return
    cta.href = '/auth/cas/login'
    cta.classList.remove('hidden')
    if (ctaText) ctaText.textContent = '登录并前往答题'
  }

  function showAuthedCta (text) {
    if (!cta) return
    cta.href = '/contest'
    cta.classList.remove('hidden')
    if (ctaText) ctaText.textContent = text || '前往答题'
    if (noTries) noTries.classList.add('hidden')
  }

  if (document.body.dataset.authenticated === 'true') {
    fetch('/api/scores', { credentials: 'same-origin' })
      .then(function (r) { return r.json().then(function (body) { return { ok: r.ok, body: body } }) })
      .then(function (res) {
        if (!res.ok) throw new Error('读取答题记录失败')
        if (res.body.attempts_left > 0) {
          showAuthedCta()
        } else {
          if (cta) cta.classList.add('hidden')
          if (noTries) noTries.classList.remove('hidden')
        }
      })
      .catch(function () { showAuthedCta() })
  } else {
    showGuestCta()
  }

  if (form) {
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
        .catch(function (err) { msg.textContent = err.message; msg.className = 'mt-2 text-sm text-red-700' })
    })
  }
}())
