const fs = require('fs')
const path = require('path')
const read = require('fs-readdir-recursive')
const marked = require('marked')

const HELP_FOLDER = path.join(__dirname, '..', 'help')

// input markdown string
// return an array of chunks each with:
// * text
// * language
// * type (html | code)
const processMarkdown = (content) => {
  const lines = content.split("\n")
  const chunks = []
  let currentChunk = null
  let codeMode = false

  lines.forEach(line => {
    if(line.indexOf('```') == 0) {
      // end of code block
      if(codeMode) {
        chunks.push(currentChunk)
        currentChunk = null
        codeMode = false
      }
      // start of code block
      else {
        if(currentChunk) {
          chunks.push(currentChunk)  
        }
        currentChunk = {
          type: 'code',
          language: line.replace('```', ''),
          text: ''
        }
        codeMode = true
      }
    }
    else {
      if(!currentChunk) {
        currentChunk = {
          type: 'html',
          text: line + "\n"
        }
      }
      else {
        currentChunk.text += line + "\n"
      }
    }
  })
  if(currentChunk) {
    chunks.push(currentChunk)
  }
  return chunks.map(chunk => {
    const finaltext = chunk.text.replace(/^\n+/, '')
    let ret = Object.assign({}, chunk, {
      text: finaltext
    })
    if(ret.type=='html') {
      ret.html = marked(ret.text)
    }
    return ret
  })
}

const files = 
  read(HELP_FOLDER)
    .filter(name => name.indexOf('.md') > 0)
    .map(name => {
      const filePath = path.join(HELP_FOLDER, name)
      const fileContent = fs.readFileSync(filePath, 'utf8')
      const content = processMarkdown(fileContent)
      return {
        name,
        content
      }
    })
    .reduce((all, file) => {
      all[file.name] = file.content
      return all
    }, {})

console.log(JSON.stringify(files, null, 4))
