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

  function csrfToken () {
    var m = document.cookie.match(/(?:^|; )contest_csrf=([^;]*)/)
    return m ? decodeURIComponent(m[1]) : ''
  }

  function showError (msg) {
    errorBox.style.display = 'block'
    errorBox.textContent = String(msg)
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

  function render () {
    form.textContent = ''
    exam.questions.forEach(function (q, index) {
      var fs = document.createElement('fieldset')
      var legend = document.createElement('legend')
      legend.textContent = (index + 1) + '. ' + q.content + '（' + categoryLabel(q.category) + '，5分）'
      fs.appendChild(legend)
      q.choices.forEach(function (c) {
        var label = document.createElement('label')
        label.className = 'choice'
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
        label.appendChild(document.createTextNode(c.content))
        fs.appendChild(label)
      })
      form.appendChild(fs)
    })
    updateProgress()
  }

  function categoryLabel (c) {
    if (c === 'B') return '判断'
    if (c === 'R') return '单选'
    if (c === 'M') return '多选'
    return c
  }

  function updateProgress () {
    var n = 0
    if (exam) {
      exam.questions.forEach(function (q) { if (collectChoices(q).length > 0) n++ })
    }
    document.getElementById('progress-text').textContent = String((exam ? exam.questions.length : 0) - n)
    document.getElementById('progress-bar').style.width = exam && exam.questions.length ? (100 * n / exam.questions.length) + '%' : '0%'
  }

  function updateTimer () {
    if (!exam) return
    var deadlineMS = exam.deadline_unix_ms || new Date(exam.deadline).getTime()
    var left = Math.max(0, deadlineMS - Date.now())
    document.getElementById('deadline-text').textContent = '剩余时间：' + Math.ceil(left / 1000) + ' 秒'
    var duration = exam.deadline_seconds || 300
    document.getElementById('time-bar').style.width = Math.min(100, Math.max(0, 100 - left / 1000 / duration * 100)) + '%'
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
      document.getElementById('attempt-title').textContent = '第 ' + exam.attempt_no + ' 次答题'
      render()
      updateTimer()
      setInterval(updateTimer, 200)
      submitBtn.disabled = false
    }).catch(function (err) {
      if (err.message === '未登录') return
      showError(err.message)
      document.getElementById('attempt-title').textContent = '无法开始答题'
    })
  }

  submitBtn.addEventListener('click', function () {
    if (submitting) return
    modal.classList.add('show')
  })
  document.getElementById('cancel-submit').addEventListener('click', function () { modal.classList.remove('show') })
  document.getElementById('confirm-submit').addEventListener('click', function () {
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
