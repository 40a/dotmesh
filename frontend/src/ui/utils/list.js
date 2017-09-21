const sortObjectList = (arr, extractor) => {
  const compare = (obja,objb) => {
    const a = extractor(obja)
    const b = extractor(objb)
    if (a < b)
      return -1
    if (a > b)
      return 1
    return 0
  }
  arr.sort(compare)
  return arr
}

const listUtils = {
  sortObjectList
}

export default listUtils