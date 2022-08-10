import { useObserver } from 'mobx-react-lite';
import React, { useEffect } from 'react';
import { NoContent, Pagination } from 'UI';
import { useStore } from 'App/mstore';
import { getRE, filterList } from 'App/utils';
import MetricListItem from '../MetricListItem';
import { sliceListPerPage } from 'App/utils';
import AnimatedSVG, { ICONS } from 'Shared/AnimatedSVG/AnimatedSVG';
import { IWidget } from 'App/mstore/types/widget';

function MetricsList() {
    const { metricStore } = useStore();
    const metrics = useObserver(() => metricStore.metrics);
    const metricsSearch = useObserver(() => metricStore.metricsSearch);

    const filterByDashboard = (item: IWidget, searchRE: RegExp) => {
        const dashboardsStr = item.dashboards.map((d: any) => d.name).join(' ')
        return searchRE.test(dashboardsStr)
    }
    const list = metricsSearch !== '' 
    ? filterList(metrics, metricsSearch, ['name', 'metricType', 'owner'], filterByDashboard) 
    : metrics;
    const lenth = list.length;

    useEffect(() => {
        metricStore.updateKey('sessionsPage', 1);
    }, [])

    return useObserver(() => (
        <NoContent
            show={lenth === 0}
            title={
                <div className="flex flex-col items-center justify-center">
                    <AnimatedSVG name={ICONS.NO_RESULTS} size={170} />
                    <div className="mt-6 text-2xl">No data available.</div>
                </div>
            }   
        >
            <div className="mt-3 border-b rounded bg-white">
                <div className="grid grid-cols-12 p-4 font-medium">
                    <div className="col-span-3">Title</div>
                    <div className="col-span-3">Owner</div>
                    <div  className="col-span-4">Visibility</div>
                    <div className="col-span-2 text-right">Last Modified</div>
                </div>

                {sliceListPerPage(list, metricStore.page - 1, metricStore.pageSize).map((metric: any) => (
                    <React.Fragment key={metric.metricId}>
                        <MetricListItem metric={metric} />
                    </React.Fragment>
                ))}
            </div>

            <div className="w-full flex items-center justify-between pt-4">
                <div className="text-disabled-text">
                    Showing <span className="font-semibold">{Math.min(list.length, metricStore.pageSize)}</span> out of <span className="font-semibold">{list.length}</span> metrics
                </div>
                <Pagination
                    page={metricStore.page}
                    totalPages={Math.ceil(lenth / metricStore.pageSize)}
                    onPageChange={(page) => metricStore.updateKey('page', page)}
                    limit={metricStore.pageSize}
                    debounceRequest={100}
                />
            </div>
        </NoContent>
    ));
}

export default MetricsList;
