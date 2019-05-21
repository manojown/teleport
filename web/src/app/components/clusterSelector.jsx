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
import siteGetters from 'app/flux/sites/getters';
import appGetters from 'app/flux/app/getters';
import DropDown from './dropdown.jsx';
import { setSiteId, refresh } from 'app/flux/app/actions';
import { isUUID } from 'app/lib/objectUtils';
import { connect } from 'nuclear-js-react-addons';

class ClusterSelector extends React.Component {
    
  onChangeSite = value => {
    setSiteId(value);      
    refresh();
  }

  render() {
    const { sites, siteId } = this.props;    
    const siteOptions = sites.map(s => ({
      label: s.name,
      value: s.name
    }));
    
    if (siteOptions.length === 1 && isUUID(siteOptions[0].value)){
      siteOptions[0].label = location.hostname;
    }
        
    return (                  
      <div className="grv-clusters-selector">   
        <div className="m-r-sm">Cluster:</div>
        <DropDown
          className="m-r-sm"
          size="sm"      
          align="right"
          onChange={this.onChangeSite}
          value={siteId}
          options={siteOptions}
          />                      
      </div>                                                                              
    );
  }
}

function mapStateToProps() {
  return {    
    sites: siteGetters.sites,
    siteId: appGetters.siteId
  }
}

export default connect(mapStateToProps)(ClusterSelector);


