import React, { Component, PropTypes } from 'react'
import { Table, TableHead, TableRow, TableCell } from 'react-toolbox/lib/table'

class ServerTable extends Component {
  render() {
    const data = this.props.data || []
    return (
      <div id="serverPage">
        <Table selectable={ false } style={{ marginTop: 10 }}>
          <TableHead>
            <TableCell>ID</TableCell>
            <TableCell>Address</TableCell>
          </TableHead>
          {data.map((item, idx) => (
            <TableRow key={idx}>
              <TableCell>{item.Id || ''}</TableCell>
              <TableCell>{(item.Addresses || []).join(', ')}</TableCell>
            </TableRow>
          ))}
        </Table>
      </div>
    )
  }
}

export default ServerTable