(function () {
  'use strict'

  var exam = null
  var pending = {}
  var timers = {}
  var submitting = false
  var autoSubmitted = false
  var form = document.getElementById('exam-form')
  var errorBox = document.getElementById('error')
  var statusBox = document.getElementById('save-status')
  var submitBtn = document.getElementById('submit-btn')
  var modal = document.getElementById('modal')
  var contestText = document.getElementById('contest-progress-text')
  var contestBar = document.getElementById('contest-progress-bar')
  var timeText = document.getElementById('time-progress-text')
  var timeBar = document.getElementById('time-progress-bar')

  function csrfToken () {
    var m = document.cookie.match(/(?:^|; )contest_csrf=([^;]*)/)
    return m ? decodeURIComponent(m[1]) : ''
  }

  function showError (msg) {
    errorBox.classList.remove('hidden')
    errorBox.textContent = String(msg)
  }

  function hideError () {
    errorBox.classList.add('hidden')
    errorBox.textContent = ''
  }

  function clearPending (qid) {
    if (pending[qid]) {
      var body = pending[qid]
      delete pending[qid]
      return body
    }
  }

  function sendAnswer (qid) {
    var body = clearPending(qid)
    if (!body) return Promise.resolve(true)
    return fetch('/api/answer', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken() },
      body: JSON.stringify(body)
    }).then(function (r) {
      if (r.status === 204) return true
      return r.json().then(function (b) { throw new Error(b.error || ('HTTP ' + r.status)) })
    }).catch(function (err) {
      showError('保存失败：' + err.message)
      return false
    })
  }

  function scheduleAnswer (qid, choiceIDs) {
    pending[qid] = { question_id: qid, choice_ids: choiceIDs }
    clearTimeout(timers[qid])
    timers[qid] = setTimeout(function () { sendAnswer(qid) }, 350)
  }

  function collectChoices (q) {
    var inputs = form.querySelectorAll('input[data-qid="' + q.id + '"]')
    var ids = []
    inputs.forEach(function (el) { if (el.checked) ids.push(Number(el.value)) })
    return ids
  }

  function scorePerQuestion () {
    if (!exam || !exam.questions.length) return 5
    return Math.round(exam.total_score / exam.questions.length)
  }

  function render () {
    form.textContent = ''
    exam.questions.forEach(function (q, index) {
      var fs = document.createElement('fieldset')
      fs.className = 'my-4 p-4 sm:px-6 lg:px-8 bg-white shadow'

      var heading = document.createElement('p')
      heading.className = 'text-lg my-0'
      var num = document.createElement('span')
      num.className = 'text-gray-600'
      num.textContent = (index + 1) + '. '
      var legend = document.createElement('legend')
      legend.className = 'inline'
      var title = document.createElement('span')
      title.className = 'font-bold'
      title.textContent = q.content
      var score = document.createElement('span')
      score.className = 'text-gray-600'
      score.textContent = '（' + scorePerQuestion() + '分）'
      legend.appendChild(title)
      legend.appendChild(score)
      heading.appendChild(num)
      heading.appendChild(legend)
      fs.appendChild(heading)

      q.choices.forEach(function (c) {
        var label = document.createElement('label')
        label.className = 'block hover:bg-red-100 my-2'
        var input = document.createElement('input')
        input.type = q.category === 'M' ? 'checkbox' : 'radio'
        input.name = 'q-' + q.id
        input.value = String(c.id)
        input.setAttribute('data-qid', String(q.id))
        var selected = (exam.answers && exam.answers[q.id]) || []
        if (selected.indexOf(c.id) >= 0) input.checked = true
        input.addEventListener('change', function () {
          scheduleAnswer(q.id, collectChoices(q))
          updateProgress()
        })
        label.appendChild(input)
        label.appendChild(document.createTextNode(' ' + c.content))
        fs.appendChild(label)
      })
      form.appendChild(fs)
    })
    updateProgress()
  }

  function updateProgress () {
    if (!exam) return
    var n = 0
    exam.questions.forEach(function (q) { if (collectChoices(q).length > 0) n++ })
    contestText.textContent = String(exam.questions.length - n)
    contestBar.style.width = exam.questions.length ? (100 * n / exam.questions.length) + '%' : '0%'
  }

  function updateTimer () {
    if (!exam) return
    var deadlineMS = exam.deadline_unix_ms || new Date(exam.deadline).getTime()
    var duration = exam.deadline_seconds || 300
    var left = Math.max(0, deadlineMS - Date.now())
    var progress = Math.min(1, Math.max(0, 1 - left / 1000 / duration))
    timeBar.style.width = (100 * progress) + '%'
    if (left > 100) {
      timeText.textContent = Math.round(left / 1000 / 60) + '分钟'
    } else {
      timeText.textContent = '约' + Math.round(left / 1000) + '秒'
    }
    if (progress > 0.85) {
      timeBar.classList.add('time-progress-severe')
    }
    if (left <= 0 && !autoSubmitted) {
      autoSubmitted = true
      doSubmit(true)
    }
  }

  function doSubmit (auto) {
    if (submitting) return
    submitting = true
    submitBtn.disabled = true
    statusBox.textContent = auto ? '时间到，正在自动提交……' : '正在提交……'
    var bodies = []
    Object.keys(pending).forEach(function (qid) {
      bodies.push(sendAnswer(Number(qid)))
    })
    Promise.all(bodies).then(function (results) {
      if (results.some(function (ok) { return !ok })) throw new Error('部分答案保存失败，请检查网络后重试')
      return fetch('/api/submit', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': csrfToken() }
      })
    }).then(function (r) {
      return r.json().then(function (b) {
        if (!r.ok) throw new Error(b.error || ('HTTP ' + r.status))
        window.location.href = '/info'
      })
    }).catch(function (err) {
      submitting = false
      submitBtn.disabled = false
      statusBox.textContent = ''
      showError('提交失败：' + err.message)
    })
  }

  function loadExam () {
    fetch('/api/exam', { credentials: 'same-origin' }).then(function (r) {
      if (r.status === 401) {
        window.location.href = '/'
        throw new Error('未登录')
      }
      return r.json().then(function (b) { return { ok: r.ok, status: r.status, body: b } })
    }).then(function (res) {
      if (!res.ok) throw new Error(res.body.error || ('HTTP ' + res.status))
      exam = res.body
      render()
      updateTimer()
      setInterval(updateTimer, 200)
      hideError()
      submitBtn.disabled = false
    }).catch(function (err) {
      if (err.message === '未登录') return
      showError(err.message)
    })
  }

  if (submitBtn) {
    submitBtn.addEventListener('click', function () {
      if (submitting) return
      if (modal) modal.classList.add('show')
    })
  }
  var cancelSubmit = document.getElementById('cancel-submit')
  if (cancelSubmit) cancelSubmit.addEventListener('click', function () { modal.classList.remove('show') })
  var confirmSubmit = document.getElementById('confirm-submit')
  if (confirmSubmit) confirmSubmit.addEventListener('click', function () {
    modal.classList.remove('show')
    doSubmit(false)
  })

  window.addEventListener('pagehide', function () {
    Object.keys(pending).forEach(function (qid) {
      var body = pending[qid]
      if (!body) return
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
}())
