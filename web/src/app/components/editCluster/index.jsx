/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import connect from '../connect';

import getters from '../../flux/settings/getters';
import { Column, Cell, TextCell, SortHeaderCell, SortTypes, EmptyIndicator } from 'app/components/table/table.jsx';

import { PagedTable } from './../table/pagedTable.jsx';

class EditCluster extends React.Component {      
  

  componentDidMount(){    
    
  }
  onSortChange = (e) => {
    
  }
  

  render(){   
    var data=  ["Welcome"];
    var searchValue = ["Search value"];
         
    return (
      <div className="grv-nodes m-t">
        <div className="grv-flex grv-header" style={{ justifyContent: "space-between" }}>
          <h2 className="text-center no-margins"> Nodes </h2>
          <div className="grv-flex">
          
          </div>
        </div>
       <div className="m-t">
      {
        // data.length === 0 && this.state.filter.length > 0 ? <EmptyIndicator text="No matching nodes found"/> :
        <PagedTable className="grv-nodes-table" tableClass="table-striped" data={data} pageSize={100}>
          <Column
            columnKey="hostname"
            header={
              <SortHeaderCell
                sortDir="Start"
                title="Hostname1"
              />
            }
            cell={<TextCell /> }
          />
          <Column
            columnKey="addr"
            header={
              <SortHeaderCell
                sortDir="asd"
                title="Address"
              />
            }
            cell={<TextCell /> }
          />
          <Column
            columnKey="alias"
            header={
              <SortHeaderCell
                sortDir="name"
                title="Alias"
              />
            }
            cell={<TextCell /> }
          />
        
            <Column
            className="grv-nodes-table-login"
          
            header={<Cell>Login as</Cell> }
          
          />
        </PagedTable>
      }
    </div>
  </div>)
  }
}

function mapStateToProps() {
  return {    
    store: getters.store    
  }
}

export default connect(mapStateToProps)(EditCluster);