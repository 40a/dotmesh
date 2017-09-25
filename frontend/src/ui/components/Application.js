import React, { Component, PropTypes } from 'react'

import { Layout, NavDrawer, Sidebar, Panel } from 'react-toolbox/lib/layout'
import AppBar from 'react-toolbox/lib/app_bar'
import SnackBar from 'react-toolbox/lib/snackbar'
import ListMenu from 'template-ui/lib/components/ListMenu'
import IconMenu from 'template-ui/lib/components/IconMenu'

import UserChip from './widgets/UserChip'

import config from '../config'

import appBarTheme from './theme/appBar.css'

class ApplicationComponent extends Component {
  render() {
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

    const title = (
      <div className={ appBarTheme.title } onClick={ this.props.toggleMenu }>
        <div className={ appBarTheme.titleContent }>
          <img src={ config.images.appbar } />
          <div id="appBarTitle">
            { this.props.title }
          </div>
        </div>
      </div>
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
          className={ appBarTheme.appBar }
          fixed
          leftIcon={ this.props.leftIcon || 'menu' }
          onLeftIconClick={ this.props.toggleMenu }
          title={ title }
        >
          <div>
            <div className={ appBarTheme.rightContent } id="appBarMenu">
              { appbarMenu }
            </div>
            {
              this.props.user ? (
                <div className={ appBarTheme.rightContent }>
                  <UserChip user={ this.props.user } />
                </div>
              ) : null
            }
          </div>
        </AppBar>
        <Panel bodyScroll={ bodyScroll }>
          {
            this.props.initialized ?
              this.props.children : 
              (
                <div></div>
              ) 
          }
        </Panel>
        <SnackBar
          active={this.props.message ? true : false}
          label={this.props.message}
          timeout={5000}
          onTimeout={this.props.clearMessage}
        />
      </Layout>
    )
  }
}

export default ApplicationComponent