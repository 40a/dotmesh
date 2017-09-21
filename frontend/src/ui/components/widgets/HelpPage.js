import React, { Component, PropTypes } from 'react'
import SyntaxHighlight from './SyntaxHighlight'
import theme from './theme/helppage.css'

import utils from '../../utils/help'
import pages from '../../help.json'

class Markdown extends Component {
  render() {
    const page = pages[this.props.page]
    if(!page) return null
    return (
      <div className={ theme.container }>
        {
          page.map((item, i) => {
            if(item.type == 'html') {
              const html = utils.processVariables(item.html, this.props.variables)
              return (
                <div key={ i } dangerouslySetInnerHTML={{__html:html}} />
              )
            }
            else if(item.type == 'code') {
              const code = utils.processVariables(item.code, this.props.variables)
              return (
                <SyntaxHighlight key={ i } language={item.language}>{code}</SyntaxHighlight>
              )
            }
            else {
              return null
            }
          }).filter(i => i)
        }
      </div>
    )
  }
}

export default Markdown 
