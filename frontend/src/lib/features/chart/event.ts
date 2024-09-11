
export function initializeEventListeners(chartContainer:HTMLElement) {
    chartContainer.addEventListener('contextmenu', (event:MouseEvent) => {
        event.preventDefault();
        const dt = new Date(1000*latestCrosshairPositionTime);
        const datePart = dt.toLocaleDateString('en-CA'); // 'en-CA' gives you the yyyy-mm-dd format
        const timePart = dt.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
        const formattedDate = `${datePart} ${timePart}`;
        const ins: Instance = { ...get(chartQuery), datetime: formattedDate, }
        queryInstanceRightClick(event,ins,"chart")
    })
    chartContainer.addEventListener('keyup', event => {
        if (event.key == "Shift"){
            shiftDown = false
        }
    })
    function shiftOverlayTrack(event:MouseEvent):void{
        shiftOverlay.update((v:ShiftOverlay) => {
            const god = {

                ...v,
                width: Math.abs(event.clientX - v.startX),
                height: Math.abs(event.clientY - v.startY),
                x: Math.min(event.clientX, v.startX),
                y: Math.min(event.clientY, v.startY),
                currentPrice: mainChartCandleSeries.coordinateToPrice(event.clientY) || 0,
            }
            console.log(god)
            return god
        })
    }
    chartContainer.addEventListener('mouseup', (event) => {
        if (queuedForwardLoad != null){
            queuedForwardLoad()
        }
    })
    chartContainer.addEventListener('mousedown',event  => {
        console.log(get(shiftOverlay))
        if (shiftDown || get(shiftOverlay).isActive){
            shiftOverlay.update((v:ShiftOverlay) => {
                v.isActive = !v.isActive
                if (v.isActive){
                    v.startX = event.clientX
                    v.startY = event.clientY
                    v.width = 0
                    v.height = 0
                    v.x = v.startX
                    v.y = v.startY
                    v.startPrice = mainChartCandleSeries.coordinateToPrice(v.startY) || 0
                    chartContainer.addEventListener("mousemove",shiftOverlayTrack)
                }else{
                    chartContainer.removeEventListener("mousemove",shiftOverlayTrack)
                }
                return v
            })
        }
    })
    chartContainer.addEventListener('keydown', event => {
        if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
            queryInstanceInput("any",get(chartQuery))
            .then((v:Instance)=>{
                changeChart(v)
            })
        }else if (event.key == "Shift"){
            shiftDown = true
        }else if (event.key == "Escape"){
            if (get(shiftOverlay).isActive){
                shiftOverlay.update((v:ShiftOverlay) => {
                    if (v.isActive){
                        v.isActive = false
                        return {
                            ...v,
                            isActive: false
                        }
                    }
                 });
    }
        }
    })

}
