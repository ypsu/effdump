// Note about variable conventions: h_ prefixed vars are UI elements defined in header.html.

function main() {
  let h = ''
  for (let name in diffs) {
    h += `<p>${name}</p>`
    h += `<table>\n`
    let x = diffs[name].lt
    let y = diffs[name].rt
    let xi = 0
    let yi = 0
    for (let i = 0; i < diffs[name].ops.length; i += 3) {
      let del = diffs[name].ops[i]
      let add = diffs[name].ops[i + 1]
      let keep = diffs[name].ops[i + 2]
      if (del > 0 || add > 0) {
        let common = Math.min(add, del)
        if (common > 0) {
          h += `<td class="cSide cbgNegative">`
          for (let ex = xi + common; xi < ex; xi++) h += lines[x[xi]]
          h += `<td class="cSide cbgPositive">`
          for (let ey = yi + common; yi < ey; yi++) h += lines[y[yi]]
          h += '<tr>\n'
        }
        if (del > common || add > common) {
          if (del == common) {
            h += `<td class="cSide cbgNeutral">`
            for (let i = del - common; i > 0; i--) h += '\n'
          } else {
            h += `<td class="cSide cbgNegative">`
            for (let ex = xi + del - common; xi < ex; xi++) h += lines[x[xi]]
          }
          if (add == common) {
            h += `<td class="cSide cbgNeutral">`
            for (let i = add - common; i > 0; i--) h += '\n'
          } else {
            h += `<td class="cSide cbgPositive">`
            for (let ey = yi + add - common; yi < ey; yi++) h += lines[y[yi]]
          }
          h += '<tr>\n'
        }
      }
      if (keep > 0) {
        h += `<td class=cSide>`
        for (let ex = xi + keep; xi < ex; xi++) h += lines[x[xi]]
        h += `<td class=cSide>`
        for (let ey = yi + keep; yi < ey; yi++) h += lines[y[yi]]
      }
      h += '<tr>\n'
    }
    h += `</table>`
    h += `<hr>`
  }

  h_UI.innerHTML = h
}
