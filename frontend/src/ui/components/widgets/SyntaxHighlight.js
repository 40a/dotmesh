import React, { Component, PropTypes } from 'react'

import SyntaxHighlighter, { registerLanguage } from "react-syntax-highlighter/dist/light"
import bash from 'react-syntax-highlighter/dist/languages/bash'
import monokai from 'react-syntax-highlighter/dist/styles/monokai-sublime'

import theme from './theme/syntaxhighlight.css'

const LANGUAGES = {
  bash
}

Object.keys(LANGUAGES).forEach(name => {
  registerLanguage(name, LANGUAGES[name])
})

class SyntaxHighlight extends Component {
  render() {
    const lang = LANGUAGES[this.props.language]
    if(!lang) throw new Error(`${lang} language not found`)
    return (
      <div className={ theme.container }>
        <SyntaxHighlighter 
          codeTagProps={{className:theme.codeContainer}}
          wrapLines={ true }
          language={ this.props.language }
          style={ monokai }
        >
          { this.props.children }
        </SyntaxHighlighter>
      </div>
    )
  }
}

export default SyntaxHighlight