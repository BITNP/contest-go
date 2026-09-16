// 历史成绩页：展示最高分、剩余次数与答题记录。
import { api } from './common.js'

function renderScores (body) {
  document.getElementById('max-score').textContent = String(body.max_score)
  document.getElementById('total-score').textContent = String(body.total_score || 100)

  const maxTries = body.max_tries || 2
  const nScores = body.scores ? body.scores.length : 0
  const attemptsLeft = document.getElementById('attempts-left')
  const goContestWrap = document.getElementById('go-contest-wrap')
  if (body.attempts_left > 0) {
    attemptsLeft.textContent = '限答' + maxTries + '次，您已答完' + nScores + '次，还有' + body.attempts_left + '次机会。'
    goContestWrap.classList.remove('hidden')
  } else {
    attemptsLeft.textContent = '限答' + maxTries + '次，您已全部答完。'
    goContestWrap.classList.add('hidden')
  }

  const tbody = document.getElementById('score-rows')
  tbody.textContent = ''
  if (!body.scores || body.scores.length === 0) return

  document.getElementById('score-section').classList.remove('hidden')
  for (const s of body.scores) {
    const tr = document.createElement('tr')
    const when = document.createElement('td')
    when.textContent = '第' + s.attempt_no + '次 · ' + new Date(s.submitted_at).toLocaleString()
    const score = document.createElement('td')
    score.textContent = String(s.score)
    tr.appendChild(when)
    tr.appendChild(score)
    tbody.appendChild(tr)
  }
}

async function main () {
  try {
    renderScores(await api('/api/scores'))
  } catch (err) {
    if (err.message === '登录已失效') return
    document.getElementById('info-error').textContent = err.message
  }
}

main()
