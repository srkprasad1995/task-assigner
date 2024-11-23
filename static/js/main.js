document.getElementById('uploadForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const formData = new FormData(e.target);
    
    try {
        const response = await fetch('/upload', {
            method: 'POST',
            body: formData
        });
        
        const timelineData = await response.json();
        
        // Create timeline
        const container = document.getElementById('timeline');
        const items = new vis.DataSet(timelineData);
        
        const options = {
            height: '500px',
            start: new Date(),
            end: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000) // 30 days from now
        };
        
        new vis.Timeline(container, items, options);
        
    } catch (error) {
        console.error('Error:', error);
        alert('Error processing file');
    }
}); 