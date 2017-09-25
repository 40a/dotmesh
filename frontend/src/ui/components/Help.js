import React, { Component, PropTypes } from 'react'

import Section from 'template-ui/lib/components/Section'
import ListMenu from 'template-ui/lib/components/ListMenu'
import TreeContent from 'template-ui/lib/components/TreeContent'

import HelpPage from './widgets/HelpPage'
import pages from '../help.json'
import theme from './theme/help.css'

class Help extends Component {

  currentPage() {
    return this.props.currentPage || this.props.menuOptions[0][0] 
  }

  processMenu(item) {
    if(item.id == this.currentPage()) {
      item.theme = {
        itemText: theme.activePageText
      }
    }
    return item
  }

  getMenu() {
    return (
      <ListMenu
        options={ this.props.menuOptions }
        onClick={ this.props.onMenuClick }
        process={ this.processMenu.bind(this) }
      />
    )
  }

  getContent() {
    const pageName = this.props.currentPage || this.props.menuOptions[0][0]
    return (
      <HelpPage
        page={ pageName + '.md' }
        variables={ this.props.variables }
      />
    )
  }

  render() {
    return (
      <TreeContent
        menu={this.getMenu()}
      >
        <Section>
          {this.getContent()}
        </Section>
      </TreeContent>
    )
  }
}

export default Help