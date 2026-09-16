(function () {
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
    document.getElementById('attempts-left').textContent = '剩余答题次数：' + body.attempts_left
    var tbody = document.getElementById('score-rows')
    tbody.textContent = ''
    if (!body.scores || body.scores.length === 0) {
      var tr = document.createElement('tr')
      var td = document.createElement('td')
      td.colSpan = 3
      td.textContent = '暂无成绩'
      tr.appendChild(td)
      tbody.appendChild(tr)
    } else {
      body.scores.forEach(function (s) {
        var tr = document.createElement('tr')
        var a = document.createElement('td')
        a.textContent = String(s.attempt_no)
        var b = document.createElement('td')
        b.textContent = String(s.score)
        var c = document.createElement('td')
        c.textContent = new Date(s.submitted_at).toLocaleString()
        tr.appendChild(a); tr.appendChild(b); tr.appendChild(c)
        tbody.appendChild(tr)
      })
    }
  }).catch(function (err) {
    if (err.message === '未登录') return
    document.getElementById('info-error').textContent = err.message
  })
}())
