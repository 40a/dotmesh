import React, { Component, PropTypes } from 'react'
import SyntaxHighlight from './SyntaxHighlight'
import theme from './theme/markdown.css'
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
              return (
                <div key={ i } dangerouslySetInnerHTML={{__html:item.html}} />
              )
            }
            else if(item.type == 'code') {
              return (
                <SyntaxHighlight key={ i } language={item.language}>{item.code}</SyntaxHighlight>
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
