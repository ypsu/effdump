function expand(evt) {
  let n = evt.target.parentNode.parentNode
  n.hidden = true
  for (n = n.nextSibling; n != null && n.hidden; n = n.nextSibling) n.hidden = false
}

function unify(evt) {
  evt.target.hidden = true
  for (let table of document.getElementsByTagName('table')) {
    let t = ''
    let add = ''
    for (let row of table.tBodies[0].childNodes) {
      if (row.children.length == 1) {
        t += add + `<tr><td colspan=3 class="cZipped cfgNeutral">` + row.children[0].innerHTML
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
