import React, { Component, PropTypes } from 'react'

import Input from 'react-toolbox/lib/input'
import theme from './theme/searchbox'

class SearchBox extends Component {
  render() {
    return (
      <Input 
        theme={ theme }
        type='text' 
        name='search'
        hint='Search...'
        value={ this.props.value }
        onChange={ this.props.onChange }
      />
    )
  }
}

export default SearchBox