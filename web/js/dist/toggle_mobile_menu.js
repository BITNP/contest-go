(function () {
  'use strict'

  function onLogout (e) {
    e.preventDefault()
    fetch('/auth/logout', { method: 'POST', credentials: 'same-origin' })
      .catch(function () {})
      .finally(function () { window.location.href = '/' })
  }

  var logout = document.getElementById('logout-link')
  if (logout) logout.addEventListener('click', onLogout)
  document.querySelectorAll('.logout-link-mobile').forEach(function (el) {
    el.addEventListener('click', onLogout)
  })

  var toggle = document.getElementById('toggle-mobile-menu')
  var menu = document.getElementById('mobile-menu')
  if (toggle && menu) {
    var icons = toggle.querySelectorAll('svg')
    var openIcon = icons[0]
    var closeIcon = icons[1]
    toggle.addEventListener('click', function () {
      var opening = menu.classList.contains('hidden')
      menu.classList.toggle('hidden', !opening)
      if (openIcon) openIcon.classList.toggle('hidden', opening)
      if (closeIcon) closeIcon.classList.toggle('hidden', !opening)
      toggle.setAttribute('aria-expanded', opening ? 'true' : 'false')
    })
  }
}())
