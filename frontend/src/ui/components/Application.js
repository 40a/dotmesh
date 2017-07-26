import React, { Component, PropTypes } from 'react'

import { Layout, NavDrawer, Sidebar, Panel } from 'react-toolbox/lib/layout'
import { AppBar } from 'react-toolbox/lib/app_bar'
import ListMenu from 'template-ui/lib/components/ListMenu'
import IconMenu from 'template-ui/lib/components/IconMenu'

class ApplicationComponent extends Component {
  render() {
    if(!this.props.initialized) {
      return (
        <div>loading...</div>
      )
    }

    const bodyScroll = typeof(this.props.autoScroll) == 'boolean' ?
      !this.props.autoScroll :
      false

    const mainMenu = (
      <ListMenu
        options={ this.props.menuOptions }
        onClick={ this.props.onMenuClick }
      />
    )

    const appbarMenu = (
      <IconMenu
        options={ this.props.menuOptions }
        onClick={ this.props.onOptionClick }
      />
    )

    return (
      <Layout>
        <NavDrawer
          active={ this.props.menuOpen }
          onOverlayClick={ this.props.toggleMenu }
          clipped={ false }
          pinned={ false }
        >
          { mainMenu }
        </NavDrawer>
        <AppBar
          fixed
          leftIcon={ this.props.leftIcon || 'menu' }
          onLeftIconClick={ this.props.toggleMenu }
          title={ this.props.title }
        >
          { appbarMenu }
        </AppBar>
        <Panel bodyScroll={ bodyScroll }>
          { this.props.children }
        </Panel>        
      </Layout>
    )
  }
}

export default ApplicationComponent