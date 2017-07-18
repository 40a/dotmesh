import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'

class About extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col lg={12}>
            <div>
              About page
            </div>
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default About