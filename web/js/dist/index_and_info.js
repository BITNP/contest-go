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
(function () {
  'use strict'

  if (!document.getElementById('score-rows')) return

  fetch('/api/scores', { credentials: 'same-origin' }).then(function (r) {
    if (r.status === 401) {
      window.location.href = '/'
      throw new Error('未登录')
    }
    return r.json().then(function (b) { return { ok: r.ok, body: b } })
  }).then(function (res) {
    if (!res.ok) throw new Error(res.body.error || '读取失败')
    var body = res.body
    document.getElementById('max-score').textContent = String(body.max_score)
    document.getElementById('total-score').textContent = String(body.total_score || 100)

    var goContestWrap = document.getElementById('go-contest-wrap')
    var attemptsLeft = document.getElementById('attempts-left')
    var nScores = body.scores ? body.scores.length : 0
    if (body.attempts_left > 0) {
      attemptsLeft.textContent = '限答' + (body.max_tries || 2) + '次，您已答完' + nScores + '次，还有' + body.attempts_left + '次机会。'
      goContestWrap.classList.remove('hidden')
      var goText = document.getElementById('url-quiz-contest-text')
      if (goText) goText.textContent = '前往答题'
    } else {
      attemptsLeft.textContent = '限答' + (body.max_tries || 2) + '次，您已全部答完。'
      goContestWrap.classList.add('hidden')
    }

    var scoreSection = document.getElementById('score-section')
    var tbody = document.getElementById('score-rows')
    tbody.textContent = ''
    if (!body.scores || body.scores.length === 0) {
      scoreSection.classList.add('hidden')
      return
    }
    scoreSection.classList.remove('hidden')
    body.scores.forEach(function (s) {
      var tr = document.createElement('tr')
      var when = document.createElement('td')
      when.textContent = '第' + s.attempt_no + '次 · ' + new Date(s.submitted_at).toLocaleString()
      var score = document.createElement('td')
      score.textContent = String(s.score)
      tr.appendChild(when)
      tr.appendChild(score)
      tbody.appendChild(tr)
    })
  }).catch(function (err) {
    if (err.message === '未登录') return
    document.getElementById('info-error').textContent = err.message
  })
}())
