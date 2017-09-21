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

const searchObjectList = (allResults, search, extractor) => {
  if(!search) return allResults
  const useSearch = search.toLowerCase().replace(/\W/g, '')
  return allResults.filter(data => {
    const useName = (extractor(data) || '').toLowerCase().replace(/\W/g, '')
    return useName.indexOf(useSearch) >= 0
  })
}

const listUtils = {
  sortObjectList,
  searchObjectList
}

export default listUtils