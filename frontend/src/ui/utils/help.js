const processVariables = (text, variables = {}) => {
  return text.replace(/\$\{(\w+)\}/g, (match, varName) => {
    return variables[varName]
  })
}

const helpUtils = {
  processVariables
}

export default helpUtils