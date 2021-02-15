import { getBackendSrv, getTemplateSrv } from '@grafana/runtime';

import {
  DataQueryRequest,
  DataQueryResponse,
  DataSourceApi,
  DataSourceInstanceSettings,
  MutableDataFrame,
  FieldType,
} from '@grafana/data';

import { MyQuery, MyDataSourceOptions } from './types';

export class DataSource extends DataSourceApi<MyQuery, MyDataSourceOptions> {
  baseUrl?: string;

  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);

    this.baseUrl = instanceSettings.url;
  }

  async doRequest(query: MyQuery, options: DataQueryRequest<MyQuery>) {
    const timeRange = options.range;
    const system_id = getTemplateSrv().replace(query.system_id, options.scopedVars);
    const stream = getTemplateSrv()
      .replace(query.stream, options.scopedVars)
      .split(',');

    const url = this.baseUrl + '/system/' + system_id + '/sample';
    console.log(url);

    const result = await getBackendSrv().datasourceRequest({
      method: 'GET',
      url: url,
      params: {
        stream: stream,
        from: timeRange.from.toISOString(),
        to: timeRange.to.toISOString(),
      },
    });

    return result;
  }

  async query(options: DataQueryRequest<MyQuery>): Promise<DataQueryResponse> {
    const promises = options.targets.map(query =>
      this.doRequest(query, options).then(response => {
        console.log(options);
        const frame = new MutableDataFrame({
          refId: query.refId,
          fields: [
            { name: 'Time', type: FieldType.time },
            { name: query.stream!, type: FieldType.number },
          ],
        });

        if (response.data != null) {
          response.data.forEach((point: any) => {
            frame.appendRow([point.timestamp, point.value]);
          });
        }

        frame.reverse();
        console.log('frame: ', frame);

        return frame;
      })
    );

    return Promise.all(promises).then(data => ({ data }));
  }

  async testDatasource() {
    // Implement a health check for your data source.
    return {
      status: 'success',
      message: 'Success',
    };
  }
}
