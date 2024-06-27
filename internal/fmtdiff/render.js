// Note about variable conventions: h_ prefixed vars are UI elements defined in header.html.

function main() {
  let h = ''
  for (let name in diffs) {
    h += `<p>${name}</p>`
    let hlt = ''
    let hrt = ''
    let x = diffs[name].lt
    let y = diffs[name].rt
    let xi = 0
    let yi = 0
    for (let i = 0; i < diffs[name].ops.length; i += 3) {
      let del = diffs[name].ops[i]
      let add = diffs[name].ops[i + 1]
      let keep = diffs[name].ops[i + 2]
      for (let i = 0; i < del; i++) hlt += lines[x[xi++]]
      for (let i = del; i < add; i++) hlt += '\n'
      for (let i = 0; i < add; i++) hrt += lines[y[yi++]]
      for (let i = add; i < del; i++) hrt += '\n'
      while (keep-- > 0) {
        hlt += lines[x[xi++]]
        hrt += lines[y[yi++]]
      }
    }
    h += `<table>\n`
    h += `  <td><pre>${hlt}</pre>`
    h += `  <td><pre>${hrt}</pre>`
    h += `</table>`
    h += `<hr>`
  }

  h_UI.innerHTML = h
}
