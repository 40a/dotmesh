import React, { Component, PropTypes } from 'react'

import SyntaxHighlight from '../widgets/SyntaxHighlight'

import texttheme from './theme/text.css'

class NoRepos extends Component {
  render() {

    const initCodeString = "$ dm init\n$ dm clone add"
    return (
      <div>
        <p className={ texttheme.text }>
          You can now use the <b>dm</b> command to create volumes!
        </p>
        <SyntaxHighlight language='bash'>{initCodeString}</SyntaxHighlight>
      </div>
    )
  }
}

export default NoRepos 
