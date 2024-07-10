function expand(evt) {
  let n = evt.target.parentNode.parentNode
  n.hidden = true
  for (n = n.nextSibling; n != null && n.hidden; n = n.nextSibling) n.hidden = false
}
