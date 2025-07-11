// Remitlane ops console. Plain JS, no build step.
(function () {
  'use strict'

  // Filter dropdowns apply as soon as they change.
  document.querySelectorAll('select[data-autosubmit]').forEach(function (el) {
    el.addEventListener('change', function () {
      if (el.form) el.form.requestSubmit()
    })
  })

  // Flash messages fade after a while.
  var flash = document.querySelector('.flash.ok')
  if (flash) {
    setTimeout(function () { flash.classList.add('fade') }, 6000)
  }
})()
