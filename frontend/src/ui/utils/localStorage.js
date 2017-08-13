const load = (name, json = false) => {
  const ret = (localStorage.getItem(name) || '').toString()
  if(!ret || !json) return ret
  return JSON.parse(ret)
}

const save = (name, value, json = false) => {
  value = json ? JSON.stringify(value) : value
  localStorage.setItem(name, value)
}

const del = (name) => localStorage.removeItem(name)

const LocalStorage = {
  load,
  save,
  del
}

export default LocalStorage