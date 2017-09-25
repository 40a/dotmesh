import React, { Component, PropTypes } from 'react'

import Autocomplete from 'react-toolbox/lib/autocomplete'

class UserDropdown extends Component {

  render() {
    return (
      <Autocomplete
        direction="down"
        selectedPosition="above"
        label="Find user..."
        multiple={false}
        onChange={this.props.change}
        onQueryChange={this.props.queryChange}
        source={this.props.source}
        value={this.props.value}
      />
    )
  }
}

export default UserDropdown