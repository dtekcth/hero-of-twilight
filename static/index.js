async function update() {
    const errorBox = document.getElementById("errorBox");
    errorBox.hidden = true;

    // Fetch services.
    let services = [];
    try {
        const response = await fetch("api/v1/services");
        if (!response.ok) {
            throw new Error(`${response.statusText}.`);
        }
        services = await response.json();
    } catch (error) {
        const errorMessage = document.getElementById("errorMessage");
        errorMessage.textContent = "Could not load services: " + error.message;
        errorBox.hidden = false;
        return;
    }

    // Fetch categories.
    let categories = [];
    try {
        const response = await fetch("api/v1/categories");
        if (!response.ok) {
            throw new Error(`${response.statusText}.`);
        }
        categories = await response.json();
    } catch (error) {
        const errorMessage = document.getElementById("errorMessage");
        errorMessage.textContent = "Could not load categories: " + error.message;
        errorBox.hidden = false;
        return;
    }

    // Collect into categories.
    const categoryServices = new Map();
    for (const service of services) {
        categoryServices.getOrInsert(service.category, []).push(service);
    }

    const collator      = new Intl.Collator();
    const boardTemplate = document.getElementById("boardTemplate");
    const cardTemplate  = document.getElementById("cardTemplate");

    // Build boards.
    const boards = [];
    for (const category of categories) {
        const services = categoryServices.get(category.id);

        if (!services) {
            continue;
        }

        // Sort services by name.
        services.sort((a, b) => collator.compare(a.name, b.name));

        // Build cards.
        const cards = services.map(service => {
            const card = document.importNode(cardTemplate.content, true);

            const name        = card.querySelector(".name");
            const container   = card.querySelector(".card");
            const description = card.querySelector(".description");

            name.textContent        = service.name;
            container.href          = service.link;
            description.textContent = service.description;

            return card;
        });

        const board = document.importNode(boardTemplate.content, true);

        const name       = board.querySelector(".name");
        const cardHolder = board.querySelector(".cardHolder");

        name.textContent = category.name;
        cardHolder.append(...cards);

        boards.push(board);
    }

    // Replace current boards.
    const boardHolder = document.getElementById("boardHolder");
    boardHolder.replaceChildren(...boards);
}

setInterval(update, 60 * 1000);
window.onload = update;
