// expand expands the zipped blocked right after the button.
function expand(evt) {
  let n = evt.target.parentNode.parentNode
  n.hidden = true
  for (n = n.nextSibling; n != null && n.hidden; n = n.nextSibling) n.hidden = false
}

// expandall expands all zipped diff parts.
function expandall(evt) {
  evt.target.hidden = true
  for (let td of document.querySelectorAll('.cZipped button')) expand({
    target: td
  })
}

// unify converts the split diff into unified diff.
function unify(evt) {
  evt.target.hidden = true
  for (let table of document.getElementsByTagName('table')) {
    let t = ''
    let add = ''
    for (let row of table.tBodies[0].childNodes) {
      if (row.children.length == 1) {
        let hidden = ''
        if (row.hidden) hidden = 'hidden'
        t += add + `<tr ${hidden}><td colspan=3 class="cZipped cfgNeutral">` + row.children[0].innerHTML
        add = ''
        continue
      }
      if (row.children[3].className == 'cRight') {
        // Unchanged line.
        let hidden = ''
        if (row.hidden) hidden = 'hidden'
        t += add + `<tr ${hidden}>`
        t += `<td class=cNum>` + row.children[0].innerHTML
        t += `<td class=cNum>` + row.children[2].innerHTML
        t += `<td class=cUnified>` + row.children[3].innerHTML
        add = ''
      }
      if (row.children[1].className == 'cLeft cbgNegative') {
        t += `<tr>`
        t += `<td class="cNum cbgNegative">` + row.children[0].innerHTML
        t += `<td class="cNum cbgNegative">`
        t += `<td class="cUnified cbgNegative">` + row.children[1].innerHTML
      }
      if (row.children[3].className == 'cRight cbgPositive') {
        add += `<tr>`
        add += `<td class="cNum cbgPositive">`
        add += `<td class="cNum cbgPositive">` + row.children[2].innerHTML
        add += `<td class="cUnified cbgPositive">` + row.children[3].innerHTML
      }
    }
    table.innerHTML = t + add
  }
}

// This is the logic for ensuring selecting from one column in the diff is possible.
function selectSide(toenable, todisable) {
  if (!toenable.disabled) return
  toenable.disabled = false
  todisable.disabled = true
  // Clear selection to avoid spurious drag events.
  window.getSelection().empty()
}

function handleclick(evt) {
  if (evt.target.classList.contains('cRight')) selectSide(hRightSelect, hLeftSelect)
  if (evt.target.classList.contains('cLeft')) selectSide(hLeftSelect, hRightSelect)
}

hLeftSelect.disabled = true
hRightSelect.disabled = true
selectSide(hRightSelect, hLeftSelect)
