// Note about variable conventions:
//
// - h_ prefixed vars are UI elements defined in header.html.
// - d_ prefixed vars come from the generated data section.
// - everything else is standard library or from this file.

function main() {
  let h = ''
  for (let name in diffs) {
    h += `<p>${name}</p>`
    h += `<pre>`
    for (let id of diffs[name].rt) {
      h += `${lines[id]}\n`
    }
    h += `</pre>`
    h += `<hr>`
  }

  h_UI.innerHTML = h
}
