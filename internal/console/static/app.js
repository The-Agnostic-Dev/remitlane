// Remitlane ops console. Plain JS, no build step.
(function () {
  'use strict'

  // Filter dropdowns apply as soon as they change.
  document.querySelectorAll('select[data-autosubmit]').forEach(function (el) {
    el.addEventListener('change', function () {
      if (el.form) el.form.requestSubmit()
    })
  })

  // Forms that do something to many records ask first.
  document.querySelectorAll('form[data-confirm]').forEach(function (form) {
    form.addEventListener('submit', function (ev) {
      if (!window.confirm(form.dataset.confirm)) ev.preventDefault()
    })
  })

  // Stop double posts: disable submit buttons once a POST form is sent.
  document.querySelectorAll('form[method="post"]').forEach(function (form) {
    form.addEventListener('submit', function (ev) {
      if (ev.defaultPrevented) return
      form.querySelectorAll('button[type="submit"], button:not([type])').forEach(function (b) {
        b.disabled = true
      })
    })
  })

  // Flash messages fade after a while.
  var flash = document.querySelector('.flash.ok')
  if (flash) {
    setTimeout(function () { flash.classList.add('fade') }, 6000)
  }
})()
